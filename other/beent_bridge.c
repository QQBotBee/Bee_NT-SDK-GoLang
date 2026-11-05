/* BeeNT x86 stdcall shell. Go runs in a separate, unloadable worker process. */
#define WIN32_LEAN_AND_MEAN
#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0600
#endif
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <plugin_config.h>

#ifndef EVENT_TIMEOUT_MS
#define EVENT_TIMEOUT_MS 200000
#endif
#ifndef STOP_TIMEOUT_MS
#define STOP_TIMEOUT_MS 3000
#endif
#ifndef WRITE_TIMEOUT_MS
#define WRITE_TIMEOUT_MS 5000
#endif
#define MAX_IPC_LINE (8u * 1024u * 1024u)
#define CALL __stdcall

typedef const char *(CALL *CoreFn)(const char *);
typedef struct Worker {
    volatile LONG references;
    volatile LONG dead;
    HANDLE pipe_read, pipe_write, process, reader, stop, reply;
    CRITICAL_SECTION write_lock;
    HMODULE module_reference;
    DWORD reader_id, sequence;
    volatile LONG pending, result;
    CoreFn core;
    WCHAR path[2048];
} Worker;

static HMODULE g_module;
static CRITICAL_SECTION g_events;
static CRITICAL_SECTION g_state;
static HANDLE g_callbacks_drained, g_callbacks_cancel;
static volatile LONG g_stopping, g_disabling;
static LONG g_active_callbacks;
static HMODULE g_callback_reference;
static Worker *g_worker;
static CoreFn g_core;
static uint32_t g_plugin_index;
static DWORD g_worker_number;
static char *g_info;

static BOOL plugin_directory(WCHAR *directory, size_t capacity);
static void bridge_log(const char *stage, DWORD error, const char *detail);

static void *allocate(size_t size) { return HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, size); }
static void release(void *value) { if (value) HeapFree(GetProcessHeap(), 0, value); }
static void close_handle(HANDLE handle) { if (handle && handle != INVALID_HANDLE_VALUE) CloseHandle(handle); }

static void release_worker(Worker *worker) {
    if (InterlockedDecrement(&worker->references) != 0) return;
    close_handle(worker->reader); close_handle(worker->process); close_handle(worker->pipe_read); close_handle(worker->pipe_write);
    close_handle(worker->stop); close_handle(worker->reply);
    DeleteCriticalSection(&worker->write_lock);
    if (worker->path[0]) DeleteFileW(worker->path);
    release(worker);
}

static Worker *worker_snapshot(void) {
    Worker *worker;
    EnterCriticalSection(&g_state);
    worker = g_worker;
    if (worker) InterlockedIncrement(&worker->references);
    LeaveCriticalSection(&g_state);
    return worker;
}

/* The host stops dispatching after disable. This guard protects callbacks that
   were already entering/waiting when disable began. Disable releases it only
   after those callbacks have returned to their exit path, while the host still
   owns its original LoadLibrary reference. */
static BOOL begin_callback(BOOL reopen) {
    BOOL ok = FALSE;
    EnterCriticalSection(&g_state);
    if (reopen && !g_disabling) { InterlockedExchange(&g_stopping, 0); ResetEvent(g_callbacks_cancel); }
    if (!g_stopping && !g_disabling) {
        if (!g_callback_reference && !GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS, (LPCWSTR)(uintptr_t)begin_callback, &g_callback_reference)) goto done;
        if (g_active_callbacks++ == 0) ResetEvent(g_callbacks_drained);
        ok = TRUE;
    }
done:
    LeaveCriticalSection(&g_state); return ok;
}

static void end_callback(void) {
    EnterCriticalSection(&g_state);
    if (--g_active_callbacks == 0) SetEvent(g_callbacks_drained);
    LeaveCriticalSection(&g_state);
}

/* All pipe operations are cancellable; callbacks never wait forever on a stopped worker. */
static BOOL pipe_io(Worker *worker, BOOL writing, void *buffer, DWORD size, DWORD *count, DWORD timeout) {
    OVERLAPPED operation; HANDLE waits[2]; BOOL ok; DWORD error, wait;
    HANDLE pipe = writing ? worker->pipe_write : worker->pipe_read;
    ZeroMemory(&operation, sizeof(operation));
    operation.hEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!operation.hEvent) return FALSE;
    *count = 0;
    ok = writing ? WriteFile(pipe, buffer, size, count, &operation) : ReadFile(pipe, buffer, size, count, &operation);
    error = ok ? ERROR_SUCCESS : GetLastError();
    if (!ok && error == ERROR_IO_PENDING) {
        waits[0] = worker->stop; waits[1] = operation.hEvent;
        wait = WaitForMultipleObjects(2, waits, FALSE, timeout);
        if (wait != WAIT_OBJECT_0 + 1) {
            CancelIoEx(pipe, &operation);
            /* Cancelled operations must finish before their OVERLAPPED storage is released. */
            GetOverlappedResult(pipe, &operation, count, TRUE);
            ok = FALSE;
        } else ok = GetOverlappedResult(pipe, &operation, count, FALSE);
    }
    CloseHandle(operation.hEvent);
    return ok && *count > 0;
}

static BOOL write_line(Worker *worker, const char *line) {
    const char *cursor = line; size_t remaining = strlen(line); BOOL ok = TRUE;
    EnterCriticalSection(&worker->write_lock);
    while (remaining && !worker->dead) {
        DWORD written = 0;
        if (!pipe_io(worker, TRUE, (void *)cursor, (DWORD)remaining, &written, WRITE_TIMEOUT_MS)) { ok = FALSE; break; }
        cursor += written; remaining -= written;
    }
    if (remaining) ok = FALSE;
    LeaveCriticalSection(&worker->write_lock);
    return ok;
}

static char *base64_encode(const unsigned char *data, size_t size) {
    static const char alphabet[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    size_t i = 0, j = 0, length = 4 * ((size + 2) / 3); char *out;
    if (length > MAX_IPC_LINE) return NULL;
    out = allocate(length + 1); if (!out) return NULL;
    while (i < size) {
        uint32_t a = data[i++], b = i < size ? data[i++] : 0, c = i < size ? data[i++] : 0;
        uint32_t bits = (a << 16) | (b << 8) | c;
        out[j++] = alphabet[(bits >> 18) & 63]; out[j++] = alphabet[(bits >> 12) & 63];
        out[j++] = alphabet[(bits >> 6) & 63]; out[j++] = alphabet[bits & 63];
    }
    if (size % 3) out[length - 1] = '=';
    if (size % 3 == 1) out[length - 2] = '=';
    return out;
}

static int base64_digit(unsigned char ch) {
    if (ch >= 'A' && ch <= 'Z') return ch - 'A';
    if (ch >= 'a' && ch <= 'z') return ch - 'a' + 26;
    if (ch >= '0' && ch <= '9') return ch - '0' + 52;
    return ch == '+' ? 62 : ch == '/' ? 63 : -1;
}

static unsigned char *base64_decode(const char *text, DWORD *size) {
    size_t length = strlen(text), i, n = 0; uint32_t bits = 0; int count = -8; unsigned char *out;
    if (length > MAX_IPC_LINE || length % 4) return NULL;
    out = allocate(length * 3 / 4 + 1); if (!out) return NULL;
    for (i = 0; i < length && text[i] != '='; i++) {
        int digit = base64_digit((unsigned char)text[i]);
        if (digit < 0) { release(out); return NULL; }
        bits = (bits << 6) | (uint32_t)digit; count += 6;
        if (count >= 0) { out[n++] = (unsigned char)(bits >> count); count -= 8; }
    }
    *size = (DWORD)n; return out;
}

/* IPC keys/IDs are generated by our Go encoder; all arbitrary text is Base64. */
static const char *json_value(const char *json, const char *key) {
    char pattern[80]; const char *p;
    snprintf(pattern, sizeof(pattern), "\"%s\"", key); p = strstr(json, pattern);
    if (!p) return NULL; p += strlen(pattern);
    while (*p == ' ' || *p == '\t') p++;
    if (*p++ != ':') return NULL;
    while (*p == ' ' || *p == '\t') p++;
    return p;
}

static char *json_string(const char *json, const char *key) {
    const char *p = json_value(json, key), *end; char *out; size_t length;
    if (!p || *p++ != '"') return NULL;
    end = strchr(p, '"'); if (!end) return NULL;
    length = (size_t)(end - p); out = allocate(length + 1);
    if (out) memcpy(out, p, length); return out;
}

static char *convert_text(const char *input, DWORD length, UINT from, UINT to) {
    int wide_length, out_length; WCHAR *wide; char *out; BOOL substituted = FALSE;
    if (!length) return allocate(1);
    wide_length = MultiByteToWideChar(from, from == CP_UTF8 ? MB_ERR_INVALID_CHARS : 0, input, (int)length, NULL, 0);
    if (!wide_length) return NULL;
    wide = allocate((size_t)wide_length * sizeof(WCHAR)); if (!wide) return NULL;
    MultiByteToWideChar(from, from == CP_UTF8 ? MB_ERR_INVALID_CHARS : 0, input, (int)length, wide, wide_length);
    out_length = WideCharToMultiByte(to, to == 936 ? WC_NO_BEST_FIT_CHARS : 0, wide, wide_length, NULL, 0, NULL, to == 936 ? &substituted : NULL);
    if (!out_length || substituted) { release(wide); return NULL; }
    out = allocate((size_t)out_length + 1);
    if (out) WideCharToMultiByte(to, to == 936 ? WC_NO_BEST_FIT_CHARS : 0, wide, wide_length, out, out_length, NULL, to == 936 ? &substituted : NULL);
    release(wide); return out;
}

static BOOL serve_api(Worker *worker, const char *line) {
    char *id = json_string(line, "id"), *encoded = json_string(line, "command_b64");
    unsigned char *utf8 = NULL; DWORD size = 0; char *command = NULL, *text = NULL, *reply_value = NULL, *reply = NULL;
    const char *answer, *error = ""; BOOL ok = FALSE;
    if (!id || !encoded || strspn(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != strlen(id)) goto done;
    utf8 = base64_decode(encoded, &size);
    if (!utf8 || memchr(utf8, 0, size) || !(command = convert_text((char *)utf8, size, CP_UTF8, 936))) error = "invalid UTF-8 or unrepresentable GBK command";
    else if (!worker->core) error = "BeeNT core address is unavailable";
    else {
        /* Return storage belongs to BeeNT. Copy before the next core call; never free it. */
        answer = worker->core(command);
        text = convert_text(answer ? answer : "", answer ? (DWORD)strlen(answer) : 0, 936, CP_UTF8);
        if (!text) error = "cannot decode BeeNT result";
    }
    reply_value = base64_encode((unsigned char *)(text ? text : ""), text ? strlen(text) : 0);
    if (!reply_value) goto done;
    reply = allocate(strlen(id) + strlen(reply_value) + strlen(error) + 100);
    if (!reply) goto done;
    sprintf(reply, "{\"type\":\"api_result\",\"id\":\"%s\",\"value_b64\":\"%s\",\"error\":\"%s\"}\n", id, reply_value, error);
    ok = write_line(worker, reply);
done:
    release(id); release(encoded); release(utf8); release(command); release(text); release(reply_value); release(reply);
    return ok;
}

static BOOL process_line(Worker *worker, const char *line) {
    char *type = json_string(line, "type"), *id = NULL; BOOL ok = FALSE;
    if (!type) return FALSE;
    if (strcmp(type, "api_call") == 0) ok = serve_api(worker, line);
    else if (strcmp(type, "event_result") == 0) {
        const char *value; char *end; unsigned long number;
        id = json_string(line, "id");
        if (id) {
            number = strtoul(id, &end, 10);
            if (*id && !*end && number && number == (DWORD)InterlockedCompareExchange(&worker->pending, 0, 0)) {
                value = json_value(line, "error");
                if (value && value[0] == '"' && value[1] != '"') bridge_log("callback_error", 0, line);
                value = json_value(line, "result");
                InterlockedExchange(&worker->result, value && atoi(value) == 1 ? 1 : 0);
                SetEvent(worker->reply);
            }
        }
        ok = TRUE;
    }
    release(type); release(id); return ok;
}

static DWORD WINAPI reader_main(void *context) {
    Worker *worker = context; size_t capacity = 4096, length = 0; char *line = allocate(capacity); BOOL ok = line != NULL;
    char chunk[4096]; HMODULE module = worker->module_reference;
    while (ok && WaitForSingleObject(worker->stop, 0) == WAIT_TIMEOUT) {
        DWORD count = 0, i;
        if (!pipe_io(worker, FALSE, chunk, sizeof(chunk), &count, INFINITE)) break;
        for (i = 0; i < count; i++) {
            if (chunk[i] == '\n') {
                line[length] = 0; ok = process_line(worker, line); length = 0;
                if (!ok) break;
            } else if (chunk[i] != '\r') {
                if (length + 1 >= capacity) {
                    char *larger;
                    if (capacity >= MAX_IPC_LINE) { ok = FALSE; break; }
                    capacity *= 2; larger = HeapReAlloc(GetProcessHeap(), 0, line, capacity);
                    if (!larger) { ok = FALSE; break; } line = larger;
                }
                line[length++] = chunk[i];
            }
        }
    }
    release(line);
    if (WaitForSingleObject(worker->stop, 0) == WAIT_TIMEOUT) {
        DWORD exit_code = STILL_ACTIVE; char detail[80];
        /* The pipe can close just before the process handle becomes signalled. */
        WaitForSingleObject(worker->process, 200);
        if (GetExitCodeProcess(worker->process, &exit_code)) {
            snprintf(detail, sizeof(detail), "exit_code=%lu", (unsigned long)exit_code);
            bridge_log(exit_code == STILL_ACTIVE ? "worker_pipe_closed" : "worker_exit", exit_code, detail);
        }
    }
    InterlockedExchange(&worker->dead, 1); SetEvent(worker->reply);
    release_worker(worker);
    FreeLibraryAndExitThread(module, 0);
    return 0;
}

static BOOL ensure_directories(const WCHAR *path) {
    WCHAR copy[2048], *p; size_t n = wcslen(path);
    if (n >= sizeof(copy) / sizeof(copy[0])) return FALSE;
    memcpy(copy, path, (n + 1) * sizeof(WCHAR));
    for (p = copy + 3; *p; p++) if (*p == L'\\') { *p = 0; CreateDirectoryW(copy, NULL); *p = L'\\'; }
    return CreateDirectoryW(copy, NULL) || GetLastError() == ERROR_ALREADY_EXISTS;
}

static BOOL plugin_directory(WCHAR *directory, size_t capacity) {
    WCHAR root[2048], *slash; int n;
    n = (int)GetModuleFileNameW(NULL, root, sizeof(root) / sizeof(root[0]));
    if (!n || n >= (int)(sizeof(root) / sizeof(root[0]))) return FALSE;
    slash = wcsrchr(root, L'\\'); if (!slash) return FALSE; *slash = 0;
    n = swprintf(directory, capacity, L"%ls\\plugin_data\\%ls", root, BEENT_PLUGIN_NAME_W);
    return n >= 0 && n < (int)capacity && ensure_directories(directory);
}

/* Only startup, explicit settings and failures are logged. Normal messages do
   not add synchronous diagnostic I/O. Preserve the failing Win32 error code. */
static void bridge_log(const char *stage, DWORD error, const char *detail) {
    DWORD saved_error = GetLastError(), written; WCHAR directory[2048], path[2200];
    HANDLE file = INVALID_HANDLE_VALUE; SYSTEMTIME time; char line[4096]; int n;
    if (plugin_directory(directory, sizeof(directory) / sizeof(directory[0]))) {
        n = swprintf(path, sizeof(path) / sizeof(path[0]), L"%ls\\bridge.log", directory);
        if (n >= 0 && n < (int)(sizeof(path) / sizeof(path[0])))
            file = CreateFileW(path, FILE_APPEND_DATA, FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    }
    if (file == INVALID_HANDLE_VALUE) {
        DWORD length = GetTempPathW(sizeof(directory) / sizeof(directory[0]), directory);
        if (!length || length >= sizeof(directory) / sizeof(directory[0])) goto done;
        n = swprintf(path, sizeof(path) / sizeof(path[0]), L"%lsbeent-bridge-%lu.log", directory, (unsigned long)GetCurrentProcessId());
        if (n < 0 || n >= (int)(sizeof(path) / sizeof(path[0]))) goto done;
        file = CreateFileW(path, FILE_APPEND_DATA, FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
        if (file == INVALID_HANDLE_VALUE) goto done;
    }
    GetLocalTime(&time);
    n = snprintf(line, sizeof(line), "%04u-%02u-%02uT%02u:%02u:%02u.%03u pid=%lu tid=%lu stage=%s win32=%lu %.*s\r\n",
        time.wYear, time.wMonth, time.wDay, time.wHour, time.wMinute, time.wSecond, time.wMilliseconds,
        (unsigned long)GetCurrentProcessId(), (unsigned long)GetCurrentThreadId(), stage, (unsigned long)error, 3000, detail ? detail : "");
    if (n > 0 && n < (int)sizeof(line)) WriteFile(file, line, (DWORD)n, &written, NULL);
    CloseHandle(file);
done:
    SetLastError(saved_error);
}

static BOOL extract_worker(Worker *worker) {
    WCHAR directory[2048]; HRSRC resource; HGLOBAL loaded; void *bytes; DWORD size, written; HANDLE file; int n;
    if (!plugin_directory(directory, sizeof(directory) / sizeof(directory[0]))) return FALSE;
    n = swprintf(worker->path, sizeof(worker->path) / sizeof(worker->path[0]), L"%ls\\beent_go_worker_%lu_%lu_%lu.exe", directory, GetCurrentProcessId(), (DWORD)(uintptr_t)g_module, ++g_worker_number);
    if (n < 0 || n >= (int)(sizeof(worker->path) / sizeof(worker->path[0]))) return FALSE;
    resource = FindResourceW(g_module, MAKEINTRESOURCEW(BEENT_WORKER_RESOURCE_ID), MAKEINTRESOURCEW(10));
    if (!resource || !(loaded = LoadResource(g_module, resource)) || !(bytes = LockResource(loaded)) || !(size = SizeofResource(g_module, resource))) return FALSE;
    file = CreateFileW(worker->path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (file == INVALID_HANDLE_VALUE) return FALSE;
    n = WriteFile(file, bytes, size, &written, NULL) && written == size;
    CloseHandle(file); return n != 0;
}

static Worker *start_worker(void) {
    Worker *worker = allocate(sizeof(Worker)); SECURITY_ATTRIBUTES attributes = {sizeof(attributes), NULL, TRUE};
    WCHAR name[160], command[2300], directory[2048], stderr_path[2200];
    HANDLE child_input = INVALID_HANDLE_VALUE, child_output = INVALID_HANDLE_VALUE, child_stderr = INVALID_HANDLE_VALUE;
    STARTUPINFOW si; PROCESS_INFORMATION pi; BOOL held = FALSE; DWORD error; const char *stage = "CreateEventW";
    if (!worker) { bridge_log("HeapAlloc", ERROR_NOT_ENOUGH_MEMORY, NULL); return NULL; }
    worker->references = 1; worker->core = g_core; worker->pipe_read = INVALID_HANDLE_VALUE; worker->pipe_write = INVALID_HANDLE_VALUE;
    InitializeCriticalSection(&worker->write_lock);
    worker->stop = CreateEventW(NULL, TRUE, FALSE, NULL); worker->reply = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!worker->stop || !worker->reply) goto fail;
    stage = "extract_worker";
    if (!extract_worker(worker)) goto fail;
    /* stdin and stdout need separate FILE_OBJECTs. A synchronous duplex child handle
       serializes Go's blocking reader and writer inside Windows and deadlocks API calls. */
    swprintf(name, sizeof(name) / sizeof(name[0]), L"\\\\.\\pipe\\beent-go-%lu-%lu-%lu-in", GetCurrentProcessId(), (DWORD)(uintptr_t)g_module, g_worker_number);
    stage = "CreateNamedPipeW(stdin)";
    worker->pipe_write = CreateNamedPipeW(name, PIPE_ACCESS_OUTBOUND | FILE_FLAG_OVERLAPPED | FILE_FLAG_FIRST_PIPE_INSTANCE, PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT, 1, 65536, 65536, 0, NULL);
    if (worker->pipe_write == INVALID_HANDLE_VALUE) goto fail;
    stage = "CreateFileW(stdin)";
    child_input = CreateFileW(name, GENERIC_READ, 0, &attributes, OPEN_EXISTING, 0, NULL);
    if (child_input == INVALID_HANDLE_VALUE) goto fail;
    swprintf(name, sizeof(name) / sizeof(name[0]), L"\\\\.\\pipe\\beent-go-%lu-%lu-%lu-out", GetCurrentProcessId(), (DWORD)(uintptr_t)g_module, g_worker_number);
    stage = "CreateNamedPipeW(stdout)";
    worker->pipe_read = CreateNamedPipeW(name, PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED | FILE_FLAG_FIRST_PIPE_INSTANCE, PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT, 1, 65536, 65536, 0, NULL);
    if (worker->pipe_read == INVALID_HANDLE_VALUE) goto fail;
    stage = "CreateFileW(stdout)";
    child_output = CreateFileW(name, GENERIC_WRITE, 0, &attributes, OPEN_EXISTING, 0, NULL);
    if (child_output == INVALID_HANDLE_VALUE) goto fail;
    stage = "CreateFileW(stderr)";
    if (!plugin_directory(directory, sizeof(directory) / sizeof(directory[0]))) goto fail;
    if (swprintf(stderr_path, sizeof(stderr_path) / sizeof(stderr_path[0]), L"%ls\\worker-stderr.log", directory) < 0) goto fail;
    child_stderr = CreateFileW(stderr_path, FILE_APPEND_DATA, FILE_SHARE_READ | FILE_SHARE_WRITE, &attributes, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (child_stderr == INVALID_HANDLE_VALUE) goto fail;
    ZeroMemory(&si, sizeof(si)); ZeroMemory(&pi, sizeof(pi));
    si.cb = sizeof(si); si.dwFlags = STARTF_USESTDHANDLES; si.hStdInput = child_input; si.hStdOutput = child_output; si.hStdError = child_stderr;
    swprintf(command, sizeof(command) / sizeof(command[0]), L"\"%ls\" --host-pid %lu --plugin-index %lu", worker->path, GetCurrentProcessId(), (unsigned long)g_plugin_index);
    stage = "CreateProcessW";
    if (!CreateProcessW(worker->path, command, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) goto fail;
    CloseHandle(child_input); child_input = INVALID_HANDLE_VALUE; CloseHandle(child_output); child_output = INVALID_HANDLE_VALUE;
    CloseHandle(child_stderr); child_stderr = INVALID_HANDLE_VALUE; CloseHandle(pi.hThread); worker->process = pi.hProcess;
    {
        char detail[80]; snprintf(detail, sizeof(detail), "worker_pid=%lu", (unsigned long)pi.dwProcessId);
        bridge_log("worker_started", 0, detail);
    }
    stage = "GetModuleHandleExW";
    if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS, (LPCWSTR)(uintptr_t)reader_main, &worker->module_reference)) goto fail;
    held = TRUE; InterlockedIncrement(&worker->references);
    stage = "CreateThread";
    worker->reader = CreateThread(NULL, 0, reader_main, worker, 0, &worker->reader_id);
    if (!worker->reader) { InterlockedDecrement(&worker->references); goto fail; }
    return worker;
fail:
    error = GetLastError(); bridge_log(stage, error, NULL);
    close_handle(child_input); close_handle(child_output); close_handle(child_stderr);
    if (worker->process) { TerminateProcess(worker->process, 1); WaitForSingleObject(worker->process, 1000); }
    if (held) FreeLibrary(worker->module_reference);
    release_worker(worker); SetLastError(error); return NULL;
}

/* g_events serializes callbacks, while the reader services API calls independently. */
static int dispatch(Worker *worker, const char *event, int argc, const char **argv, DWORD timeout) {
    size_t size = 160, used; char *line, **encoded = NULL; int i, result = 0; DWORD id, wait;
    BOOL cancellable = strcmp(event, "disable") != 0 && strcmp(event, "unload") != 0 && strcmp(event, "stop") != 0;
    HANDLE waits[2];
    if (g_disabling && strcmp(event, "initialize") == 0) cancellable = FALSE;
    if (!worker || worker->dead || GetCurrentThreadId() == worker->reader_id) return 0;
    if (cancellable && g_stopping) return 0;
    if (argc) { encoded = allocate((size_t)argc * sizeof(char *)); if (!encoded) return 0; }
    for (i = 0; i < argc; i++) {
        const char *value = argv[i] ? argv[i] : "";
        encoded[i] = base64_encode((const unsigned char *)value, strlen(value));
        if (!encoded[i]) goto done;
        size += strlen(encoded[i]) + 3;
        if (size >= MAX_IPC_LINE) goto done;
    }
    line = allocate(size); if (!line) goto done;
    id = ++worker->sequence; if (!id) id = ++worker->sequence;
    used = (size_t)sprintf(line, "{\"type\":\"event\",\"id\":\"%lu\",\"event\":\"%s\",\"args_b64\":[", id, event);
    for (i = 0; i < argc; i++) used += (size_t)sprintf(line + used, "%s\"%s\"", i ? "," : "", encoded[i]);
    strcpy(line + used, "]}\n");
    ResetEvent(worker->reply); InterlockedExchange(&worker->result, 0); InterlockedExchange(&worker->pending, (LONG)id);
    if (write_line(worker, line)) {
        waits[0] = worker->reply; waits[1] = g_callbacks_cancel;
        wait = WaitForMultipleObjects(cancellable ? 2 : 1, waits, FALSE, timeout);
        if (wait == WAIT_OBJECT_0 && !worker->dead) result = (int)worker->result;
        else { InterlockedExchange(&worker->dead, 1); SetEvent(worker->stop); }
    } else { InterlockedExchange(&worker->dead, 1); SetEvent(worker->stop); }
    InterlockedExchange(&worker->pending, 0); release(line);
done:
    for (i = 0; i < argc; i++) release(encoded ? encoded[i] : NULL);
    release(encoded); return result;
}

static void stop_worker(void) {
    Worker *worker;
    EnterCriticalSection(&g_state); worker = g_worker; g_worker = NULL; LeaveCriticalSection(&g_state);
    if (!worker) return;
    if (!worker->dead) dispatch(worker, "stop", 0, NULL, STOP_TIMEOUT_MS);
    SetEvent(worker->stop); CancelIoEx(worker->pipe_read, NULL); CancelIoEx(worker->pipe_write, NULL);
    if (WaitForSingleObject(worker->process, STOP_TIMEOUT_MS) == WAIT_TIMEOUT) TerminateProcess(worker->process, 0);
    WaitForSingleObject(worker->process, 1000);
    WaitForSingleObject(worker->reader, STOP_TIMEOUT_MS);
    /* A core API may still be running. Its thread owns this session and a DLL reference. */
    release_worker(worker);
}

static BOOL acquire_events(DWORD timeout, BOOL cancellable) {
    ULONGLONG deadline = GetTickCount64() + timeout;
    Worker *worker = worker_snapshot(); BOOL reentered = worker && GetCurrentThreadId() == worker->reader_id;
    if (worker) release_worker(worker);
    if (reentered) return FALSE;
    do {
        if (cancellable && g_stopping) return FALSE;
        if (TryEnterCriticalSection(&g_events)) return TRUE;
        Sleep(1);
    } while (GetTickCount64() < deadline);
    return FALSE;
}

static BOOL ensure_worker(DWORD timeout) {
    if (g_worker && g_worker->dead) stop_worker();
    if (!g_worker) {
        Worker *worker = start_worker(); if (!worker) return FALSE;
        EnterCriticalSection(&g_state); g_worker = worker; LeaveCriticalSection(&g_state);
        dispatch(g_worker, "initialize", 0, NULL, timeout);
        if (g_worker->dead) { stop_worker(); return FALSE; }
    }
    return TRUE;
}

static int send_event(const char *event, int argc, const char **argv) {
    int result = 0;
    if (!acquire_events(EVENT_TIMEOUT_MS, TRUE)) return 0;
    if (ensure_worker(EVENT_TIMEOUT_MS)) result = dispatch(g_worker, event, argc, argv, EVENT_TIMEOUT_MS);
    if (g_worker && g_worker->dead) stop_worker();
    LeaveCriticalSection(&g_events); return result;
}

static void CALL plugin_enable(void) { if (begin_callback(TRUE)) { send_event("enable", 0, NULL); end_callback(); } }
static void CALL plugin_setting(void) { if (begin_callback(TRUE)) { bridge_log("settings_requested", 0, NULL); send_event("settings", 0, NULL); end_callback(); } }
static void finish_lifecycle(BOOL uninstall) {
    Worker *worker; HMODULE reference = NULL; LONG active;
    EnterCriticalSection(&g_state);
    if (g_disabling) { LeaveCriticalSection(&g_state); return; }
    InterlockedExchange(&g_disabling, 1); InterlockedExchange(&g_stopping, 1);
    active = g_active_callbacks; SetEvent(g_callbacks_cancel);
    LeaveCriticalSection(&g_state);
    if (active) {
        worker = worker_snapshot();
        if (worker) {
            InterlockedExchange(&worker->dead, 1); SetEvent(worker->stop); SetEvent(worker->reply);
            CancelIoEx(worker->pipe_read, NULL); CancelIoEx(worker->pipe_write, NULL); TerminateProcess(worker->process, 0);
            release_worker(worker);
        }
    }
    if (WaitForSingleObject(g_callbacks_drained, STOP_TIMEOUT_MS * 4) != WAIT_OBJECT_0) {
        /* Retain the module guard if external filesystem/process work has stalled.
           This rare failure may defer unloading but cannot execute freed DLL code. */
        InterlockedExchange(&g_disabling, 0); return;
    }
    if (!acquire_events(STOP_TIMEOUT_MS, FALSE)) { InterlockedExchange(&g_disabling, 0); return; }
    if (uninstall) {
        if (ensure_worker(STOP_TIMEOUT_MS)) dispatch(g_worker, "unload", 0, NULL, STOP_TIMEOUT_MS);
    } else if (g_worker && !g_worker->dead) dispatch(g_worker, "disable", 0, NULL, STOP_TIMEOUT_MS);
    stop_worker(); LeaveCriticalSection(&g_events);
    EnterCriticalSection(&g_state);
    reference = g_callback_reference; g_callback_reference = NULL;
    InterlockedExchange(&g_disabling, 0);
    LeaveCriticalSection(&g_state);
    /* The caller must retain its original module reference until this returns. */
    if (reference) FreeLibrary(reference);
}
static void CALL plugin_disable(void) { finish_lifecycle(FALSE); }
static void CALL plugin_uninstall(void) { finish_lifecycle(TRUE); }
static int CALL plugin_private(const char *qq, const char *message, const char *id) {
    const char *args[] = {qq, message, id}; int result;
    if (!begin_callback(FALSE)) return 0;
    result = send_event("private_message", 3, args); end_callback(); return result;
}
static int CALL plugin_group(const char *group, const char *qq, const char *message, const char *id) {
    const char *args[] = {group, qq, message, id}; int result;
    if (!begin_callback(FALSE)) return 0;
    result = send_event("group_message", 4, args); end_callback(); return result;
}
static int CALL plugin_event(const char *type, const char *source, const char *trigger_name, const char *trigger, const char *operator_name, const char *operator_qq, const char *message, const char *flag) {
    const char *args[] = {type, source, trigger_name, trigger, operator_name, operator_qq, message, flag}; int result;
    if (!begin_callback(FALSE)) return 0;
    result = send_event("common_event", 8, args); end_callback(); return result;
}

__declspec(dllexport) const char *CALL botPluginLoad(uint32_t core, uint32_t index) {
    size_t length;
    if (!core || !acquire_events(STOP_TIMEOUT_MS, FALSE)) return "";
    if (g_worker) { LeaveCriticalSection(&g_events); return ""; }
    g_core = (CoreFn)(uintptr_t)core; g_plugin_index = index;
    release(g_info); g_info = allocate(sizeof(BEENT_PLUGIN_METADATA_JSON_GBK) + 320);
    if (!g_info) { LeaveCriticalSection(&g_events); return ""; }
    length = strlen((const char *)BEENT_PLUGIN_METADATA_JSON_GBK);
    memcpy(g_info, BEENT_PLUGIN_METADATA_JSON_GBK, length - 1);
    sprintf(g_info + length - 1, ",\"enable\":%lu,\"disable\":%lu,\"uninstall\":%lu,\"setting\":%lu,\"private\":%lu,\"group\":%lu,\"event\":%lu}",
        (unsigned long)(uintptr_t)plugin_enable, (unsigned long)(uintptr_t)plugin_disable,
        (unsigned long)(uintptr_t)plugin_uninstall, (unsigned long)(uintptr_t)plugin_setting,
        (unsigned long)(uintptr_t)plugin_private, (unsigned long)(uintptr_t)plugin_group, (unsigned long)(uintptr_t)plugin_event);
    LeaveCriticalSection(&g_events); return g_info;
}

BOOL WINAPI DllMain(HINSTANCE instance, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_module = instance; InitializeCriticalSection(&g_events); InitializeCriticalSection(&g_state);
        g_callbacks_drained = CreateEventW(NULL, TRUE, TRUE, NULL); g_callbacks_cancel = CreateEventW(NULL, TRUE, FALSE, NULL);
        if (!g_callbacks_drained || !g_callbacks_cancel) { close_handle(g_callbacks_drained); close_handle(g_callbacks_cancel); DeleteCriticalSection(&g_events); DeleteCriticalSection(&g_state); return FALSE; }
        DisableThreadLibraryCalls(instance);
    } else if (reason == DLL_PROCESS_DETACH) {
        /* No waits or thread joins under the loader lock. Proper unload follows disable. */
        if (g_worker) { SetEvent(g_worker->stop); CancelIoEx(g_worker->pipe_read, NULL); CancelIoEx(g_worker->pipe_write, NULL); }
        release(g_info); g_info = NULL; close_handle(g_callbacks_drained); close_handle(g_callbacks_cancel); DeleteCriticalSection(&g_events); DeleteCriticalSection(&g_state);
    }
    return TRUE;
}
