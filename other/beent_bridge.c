/* Bee×NT 纯 C 壳。Go 运行时在独立工作进程中，不进入框架进程。 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <plugin_config.h>

#ifndef __GNUC__
#define __attribute__(x)
#endif

#define BEENT_CALL __stdcall
#define IPC_TIMEOUT_MS 15000
#define STOP_TIMEOUT_MS 3000
#define MAX_IPC_LINE (8 * 1024 * 1024)

typedef const char *(BEENT_CALL *CoreFn)(const char *);

static HMODULE g_module;
static CRITICAL_SECTION g_lock;
static BOOL g_lock_ready;
static HANDLE g_process;
static HANDLE g_stdin_write;
static HANDLE g_stdout_read;
static DWORD g_request_id;
static CoreFn g_core;
static uint32_t g_plugin_index;
static char *g_info;
static WCHAR g_worker_path[MAX_PATH * 4];

static void *heap_alloc(size_t size) { return HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, size); }
static void heap_free(void *value) { if (value) HeapFree(GetProcessHeap(), 0, value); }

static BOOL write_all(HANDLE handle, const void *data, DWORD size) {
    const BYTE *cursor = (const BYTE *)data;
    while (size) {
        DWORD written = 0;
        if (!WriteFile(handle, cursor, size, &written, NULL) || written == 0) return FALSE;
        cursor += written;
        size -= written;
    }
    return TRUE;
}

static char *base64_encode(const unsigned char *data, size_t len) {
    static const char table[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    size_t out_len = 4 * ((len + 2) / 3), i = 0, j = 0;
    char *out = (char *)heap_alloc(out_len + 1);
    if (!out) return NULL;
    while (i < len) {
        uint32_t a = i < len ? data[i++] : 0;
        uint32_t b = i < len ? data[i++] : 0;
        uint32_t c = i < len ? data[i++] : 0;
        uint32_t triple = (a << 16) | (b << 8) | c;
        out[j++] = table[(triple >> 18) & 63];
        out[j++] = table[(triple >> 12) & 63];
        out[j++] = table[(triple >> 6) & 63];
        out[j++] = table[triple & 63];
    }
    if (len % 3) out[out_len - 1] = '=';
    if (len % 3 == 1) out[out_len - 2] = '=';
    return out;
}

static char *json_escape(const char *value) {
    size_t len = value ? strlen(value) : 0, i, out_len = 0;
    char *out = (char *)heap_alloc(len * 6 + 1);
    if (!out) return NULL;
    for (i = 0; i < len; i++) {
        unsigned char ch = (unsigned char)value[i];
        if (ch == '"' || ch == '\\') { out[out_len++] = '\\'; out[out_len++] = (char)ch; }
        else if (ch == '\n') { out[out_len++] = '\\'; out[out_len++] = 'n'; }
        else if (ch == '\r') { out[out_len++] = '\\'; out[out_len++] = 'r'; }
        else if (ch == '\t') { out[out_len++] = '\\'; out[out_len++] = 't'; }
        else if (ch < 0x20) out_len += (size_t)sprintf(out + out_len, "\\u%04x", ch);
        else out[out_len++] = (char)ch;
    }
    out[out_len] = 0;
    return out;
}

static BOOL ensure_directory_tree(const WCHAR *path) {
    WCHAR copy[MAX_PATH * 4];
    WCHAR *p;
    if (lstrlenW(path) >= (int)(sizeof(copy) / sizeof(copy[0]))) return FALSE;
    lstrcpyW(copy, path);
    for (p = copy + 3; *p; p++) {
        if (*p == L'\\' || *p == L'/') {
            WCHAR saved = *p;
            *p = 0;
            CreateDirectoryW(copy, NULL);
            *p = saved;
        }
    }
    return CreateDirectoryW(copy, NULL) || GetLastError() == ERROR_ALREADY_EXISTS;
}

/* 数据目录跟随框架 EXE，不跟随 temp_plugin 中的 DLL 副本。 */
static BOOL build_worker_path(void) {
    WCHAR root[MAX_PATH * 4];
    WCHAR directory[MAX_PATH * 4];
    DWORD n = GetModuleFileNameW(NULL, root, (DWORD)(sizeof(root) / sizeof(root[0])));
    WCHAR *slash;
    if (!n || n >= sizeof(root) / sizeof(root[0])) return FALSE;
    slash = wcsrchr(root, L'\\');
    if (!slash) return FALSE;
    *slash = 0;
    if (swprintf(directory, sizeof(directory) / sizeof(directory[0]), L"%ls\\plugin_data\\%ls", root, BEENT_PLUGIN_NAME_W) < 0) return FALSE;
    if (!ensure_directory_tree(directory)) return FALSE;
    return swprintf(g_worker_path, sizeof(g_worker_path) / sizeof(g_worker_path[0]), L"%ls\\%ls", directory, BEENT_WORKER_FILENAME) >= 0;
}

static BOOL extract_worker(void) {
    HRSRC resource;
    HGLOBAL loaded;
    const void *bytes;
    DWORD size, written = 0;
    HANDLE file;
    if (!build_worker_path()) return FALSE;
    resource = FindResourceW(g_module, MAKEINTRESOURCEW(BEENT_WORKER_RESOURCE_ID), MAKEINTRESOURCEW(10));
    if (!resource) return FALSE;
    loaded = LoadResource(g_module, resource);
    bytes = loaded ? LockResource(loaded) : NULL;
    size = SizeofResource(g_module, resource);
    if (!bytes || !size) return FALSE;
    file = CreateFileW(g_worker_path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (file == INVALID_HANDLE_VALUE) return FALSE;
    if (!WriteFile(file, bytes, size, &written, NULL) || written != size) { CloseHandle(file); return FALSE; }
    FlushFileBuffers(file);
    CloseHandle(file);
    return TRUE;
}

static void close_handle(HANDLE *handle) {
    if (*handle && *handle != INVALID_HANDLE_VALUE) { CloseHandle(*handle); *handle = NULL; }
}

static BOOL worker_running(void) {
    return g_process && WaitForSingleObject(g_process, 0) == WAIT_TIMEOUT;
}

static BOOL start_worker(void) {
    SECURITY_ATTRIBUTES sa = {sizeof(sa), NULL, TRUE};
    HANDLE child_stdin_read = NULL, child_stdout_write = NULL;
    STARTUPINFOW si;
    PROCESS_INFORMATION pi;
    WCHAR command[MAX_PATH * 4 + 96];
    if (worker_running()) return TRUE;
    if (!extract_worker()) return FALSE;
    if (!CreatePipe(&child_stdin_read, &g_stdin_write, &sa, 0)) return FALSE;
    if (!SetHandleInformation(g_stdin_write, HANDLE_FLAG_INHERIT, 0)) goto fail;
    if (!CreatePipe(&g_stdout_read, &child_stdout_write, &sa, 0)) goto fail;
    if (!SetHandleInformation(g_stdout_read, HANDLE_FLAG_INHERIT, 0)) goto fail;
    ZeroMemory(&si, sizeof(si));
    ZeroMemory(&pi, sizeof(pi));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES;
    si.hStdInput = child_stdin_read;
    si.hStdOutput = child_stdout_write;
    si.hStdError = GetStdHandle(STD_ERROR_HANDLE);
    swprintf(command, sizeof(command) / sizeof(command[0]), L"\"%ls\" --host-pid %lu --core %lu --plugin-index %lu",
        g_worker_path, GetCurrentProcessId(), (unsigned long)(uintptr_t)g_core, (unsigned long)g_plugin_index);
    if (!CreateProcessW(g_worker_path, command, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) goto fail;
    close_handle(&child_stdin_read);
    close_handle(&child_stdout_write);
    close_handle(&pi.hThread);
    g_process = pi.hProcess;
    return TRUE;
fail:
    close_handle(&child_stdin_read);
    close_handle(&child_stdout_write);
    close_handle(&g_stdin_write);
    close_handle(&g_stdout_read);
    return FALSE;
}

static BOOL read_line(char **line_out) {
    size_t capacity = 4096, length = 0;
    char *line = (char *)heap_alloc(capacity);
    if (!line) return FALSE;
    for (;;) {
        char ch;
        DWORD got = 0;
        if (!ReadFile(g_stdout_read, &ch, 1, &got, NULL) || got != 1) { heap_free(line); return FALSE; }
        if (ch == '\n') break;
        if (length + 1 >= capacity) {
            char *larger;
            if (capacity >= MAX_IPC_LINE) { heap_free(line); return FALSE; }
            capacity *= 2;
            larger = (char *)HeapReAlloc(GetProcessHeap(), 0, line, capacity);
            if (!larger) { heap_free(line); return FALSE; }
            line = larger;
        }
        if (ch != '\r') line[length++] = ch;
    }
    line[length] = 0;
    *line_out = line;
    return TRUE;
}

static const char *json_string_field(const char *json, const char *field, char **owned) {
    char pattern[64];
    const char *start, *end;
    size_t length;
    sprintf(pattern, "\"%s\":\"", field);
    start = strstr(json, pattern);
    if (!start) return NULL;
    start += strlen(pattern);
    end = strchr(start, '"');
    if (!end) return NULL;
    length = (size_t)(end - start);
    *owned = (char *)heap_alloc(length + 1);
    if (!*owned) return NULL;
    memcpy(*owned, start, length);
    (*owned)[length] = 0;
    return *owned;
}

static unsigned char *base64_decode(const char *text, DWORD *size_out) {
    static signed char map[256];
    static BOOL initialized;
    size_t len, i, out_len = 0;
    unsigned char *out;
    int value = 0, bits = -8;
    if (!initialized) {
        memset(map, -1, sizeof(map));
        for (i = 0; i < 26; i++) { map['A' + i] = (signed char)i; map['a' + i] = (signed char)(26 + i); }
        for (i = 0; i < 10; i++) map['0' + i] = (signed char)(52 + i);
        map[(unsigned char)'+'] = 62;
        map[(unsigned char)'/'] = 63;
        initialized = TRUE;
    }
    len = text ? strlen(text) : 0;
    out = (unsigned char *)heap_alloc(len * 3 / 4 + 4);
    if (!out) return NULL;
    for (i = 0; i < len && text[i] != '='; i++) {
        int decoded = map[(unsigned char)text[i]];
        if (decoded < 0) continue;
        value = (value << 6) | decoded;
        bits += 6;
        if (bits >= 0) { out[out_len++] = (unsigned char)((value >> bits) & 0xff); bits -= 8; }
    }
    out[out_len] = 0;
    *size_out = (DWORD)out_len;
    return out;
}

static BOOL utf8_to_gbk(const unsigned char *utf8, DWORD utf8_size, char **gbk_out) {
    int wide_len, gbk_len;
    WCHAR *wide;
    if (!utf8_size) {
        *gbk_out = (char *)heap_alloc(1);
        return *gbk_out != NULL;
    }
    wide_len = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, (const char *)utf8, (int)utf8_size, NULL, 0);
    if (!wide_len) return FALSE;
    wide = (WCHAR *)heap_alloc((size_t)(wide_len + 1) * sizeof(WCHAR));
    if (!wide) return FALSE;
    MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, (const char *)utf8, (int)utf8_size, wide, wide_len);
    gbk_len = WideCharToMultiByte(936, 0, wide, wide_len, NULL, 0, NULL, NULL);
    if (!gbk_len) { heap_free(wide); return FALSE; }
    *gbk_out = (char *)heap_alloc((size_t)gbk_len + 1);
    if (!*gbk_out) { heap_free(wide); return FALSE; }
    WideCharToMultiByte(936, 0, wide, wide_len, *gbk_out, gbk_len, NULL, NULL);
    heap_free(wide);
    return TRUE;
}

static BOOL gbk_to_utf8(const char *gbk, char **utf8_out, DWORD *utf8_size) {
    int wide_len, utf8_len;
    WCHAR *wide;
    if (!gbk || !gbk[0]) {
        *utf8_out = (char *)heap_alloc(1);
        *utf8_size = 0;
        return *utf8_out != NULL;
    }
    wide_len = MultiByteToWideChar(936, 0, gbk, -1, NULL, 0);
    if (wide_len <= 0) return FALSE;
    wide = (WCHAR *)heap_alloc((size_t)wide_len * sizeof(WCHAR));
    if (!wide) return FALSE;
    MultiByteToWideChar(936, 0, gbk, -1, wide, wide_len);
    utf8_len = WideCharToMultiByte(CP_UTF8, 0, wide, wide_len - 1, NULL, 0, NULL, NULL);
    if (utf8_len < 0) { heap_free(wide); return FALSE; }
    *utf8_out = (char *)heap_alloc((size_t)utf8_len + 1);
    if (!*utf8_out) { heap_free(wide); return FALSE; }
    WideCharToMultiByte(CP_UTF8, 0, wide, wide_len - 1, *utf8_out, utf8_len, NULL, NULL);
    *utf8_size = (DWORD)utf8_len;
    heap_free(wide);
    return TRUE;
}

/* 框架返回的缓冲区归框架所有，这里只复制，不释放。 */
static BOOL send_api_result(const char *id, const char *value_gbk, const char *error) {
    char *utf8 = NULL, *encoded = NULL, *escaped_error = NULL, *line = NULL;
    DWORD utf8_size = 0;
    BOOL ok = FALSE;
    if (value_gbk && !gbk_to_utf8(value_gbk, &utf8, &utf8_size)) return FALSE;
    encoded = base64_encode((const unsigned char *)(utf8 ? utf8 : ""), utf8 ? (size_t)utf8_size : 0);
    escaped_error = json_escape(error ? error : "");
    if (!encoded || !escaped_error) goto done;
    line = (char *)heap_alloc(strlen(id) + strlen(encoded) + strlen(escaped_error) + 96);
    if (!line) goto done;
    sprintf(line, "{\"type\":\"api_result\",\"id\":\"%s\",\"value_b64\":\"%s\",\"error\":\"%s\"}\n", id, encoded, escaped_error);
    ok = write_all(g_stdin_write, line, (DWORD)strlen(line));
done:
    heap_free(utf8);
    heap_free(encoded);
    heap_free(escaped_error);
    heap_free(line);
    return ok;
}

static BOOL service_api_call(const char *line) {
    char *id = NULL, *command_b64 = NULL, *gbk_command = NULL;
    unsigned char *utf8_command = NULL;
    DWORD utf8_size = 0;
    const char *result = "";
    BOOL ok = FALSE;
    if (!json_string_field(line, "id", &id) || !json_string_field(line, "command_b64", &command_b64)) goto done;
    utf8_command = base64_decode(command_b64, &utf8_size);
    if (!utf8_command || !utf8_to_gbk(utf8_command, utf8_size, &gbk_command)) {
        send_api_result(id, NULL, "invalid api command");
        goto done;
    }
    if (!g_core) {
        send_api_result(id, NULL, "BeeNT core address is unavailable");
        goto done;
    }
    result = g_core(gbk_command);
    ok = send_api_result(id, result ? result : "", NULL);
done:
    heap_free(id);
    heap_free(command_b64);
    heap_free(utf8_command);
    heap_free(gbk_command);
    return ok;
}

static int parse_result(const char *line) {
    const char *value = strstr(line, "\"result\":");
    return value ? atoi(value + 9) : 0;
}

/* 参数是框架传入的 GBK 文本，先转成 UTF-8 再交给 Go。 */
static int send_event(const char *event, int argc, const char **argv) {
    char header[256], tail[] = "]}\n";
    DWORD id;
    int i, result = 0;
    char *line = NULL;
    if (!g_lock_ready) return 0;
    EnterCriticalSection(&g_lock);
    if (!start_worker()) goto done;
    id = ++g_request_id;
    sprintf(header, "{\"type\":\"event\",\"id\":\"%lu\",\"event\":\"%s\",\"args_b64\":[", id, event);
    if (!write_all(g_stdin_write, header, (DWORD)strlen(header))) goto done;
    for (i = 0; i < argc; i++) {
        char *utf8 = NULL;
        char *encoded;
        DWORD utf8_size = 0;
        if (!gbk_to_utf8(argv[i] ? argv[i] : "", &utf8, &utf8_size)) goto done;
        encoded = base64_encode((const unsigned char *)utf8, utf8_size);
        heap_free(utf8);
        if (!encoded) goto done;
        if (i && !write_all(g_stdin_write, ",", 1)) { heap_free(encoded); goto done; }
        if (!write_all(g_stdin_write, "\"", 1) || !write_all(g_stdin_write, encoded, (DWORD)strlen(encoded)) || !write_all(g_stdin_write, "\"", 1)) {
            heap_free(encoded);
            goto done;
        }
        heap_free(encoded);
    }
    if (!write_all(g_stdin_write, tail, (DWORD)strlen(tail))) goto done;
    for (;;) {
        if (!read_line(&line)) goto done;
        if (strstr(line, "\"type\":\"api_call\"")) {
            if (!service_api_call(line)) goto done;
            heap_free(line);
            line = NULL;
            continue;
        }
        break;
    }
    result = parse_result(line);
done:
    heap_free(line);
    LeaveCriticalSection(&g_lock);
    return result;
}

static void stop_worker(const char *lifecycle) {
    if (!g_lock_ready) return;
    if (worker_running()) send_event(lifecycle, 0, NULL);
    close_handle(&g_stdin_write);
    if (g_process && WaitForSingleObject(g_process, STOP_TIMEOUT_MS) == WAIT_TIMEOUT) TerminateProcess(g_process, 0);
    if (g_process) WaitForSingleObject(g_process, 1000);
    close_handle(&g_stdout_read);
    close_handle(&g_process);
}

static void BEENT_CALL plugin_enable(void) { send_event("enable", 0, NULL); }
static void BEENT_CALL plugin_disable(void) { if (worker_running()) send_event("disable", 0, NULL); stop_worker("stop"); }
static void BEENT_CALL plugin_uninstall(void) { send_event("unload", 0, NULL); stop_worker("stop"); }
static void BEENT_CALL plugin_setting(void) { send_event("settings", 0, NULL); }
static int BEENT_CALL plugin_private(const char *qq, const char *message, const char *id) {
    const char *args[] = {qq, message, id};
    return send_event("friend_message", 3, args);
}
static int BEENT_CALL plugin_group(const char *group, const char *qq, const char *message, const char *id) {
    const char *args[] = {group, qq, message, id};
    return send_event("group_message", 4, args);
}
static int BEENT_CALL plugin_event(const char *type, const char *source, const char *trigger_name, const char *trigger,
    const char *operator_name, const char *operator_qq, const char *message, const char *flag) {
    const char *args[] = {type, source, trigger_name, trigger, operator_name, operator_qq, message, flag};
    return send_event("event_message", 8, args);
}

/* 框架按 stdcall 调用，参数为 API 入口地址和插件索引，返回 GBK JSON。 */
__declspec(dllexport) const char *BEENT_CALL BeeNT_Initialize(uint32_t core, uint32_t index) {
    size_t length;
    if (!core || !g_lock_ready) return "";
    EnterCriticalSection(&g_lock);
    g_core = (CoreFn)(uintptr_t)core;
    g_plugin_index = index;
    heap_free(g_info);
    g_info = (char *)heap_alloc(sizeof(BEENT_PLUGIN_METADATA_JSON_GBK) + 320);
    if (!g_info) { LeaveCriticalSection(&g_lock); return ""; }
    length = strlen((const char *)BEENT_PLUGIN_METADATA_JSON_GBK);
    if (length == 0 || BEENT_PLUGIN_METADATA_JSON_GBK[length - 1] != '}') { LeaveCriticalSection(&g_lock); return ""; }
    memcpy(g_info, BEENT_PLUGIN_METADATA_JSON_GBK, length - 1);
    sprintf(g_info + length - 1,
        ",\"enable\":%lu,\"disable\":%lu,\"uninstall\":%lu,\"setting\":%lu,\"private\":%lu,\"group\":%lu,\"event\":%lu}",
        (unsigned long)(uintptr_t)plugin_enable, (unsigned long)(uintptr_t)plugin_disable,
        (unsigned long)(uintptr_t)plugin_uninstall, (unsigned long)(uintptr_t)plugin_setting,
        (unsigned long)(uintptr_t)plugin_private, (unsigned long)(uintptr_t)plugin_group,
        (unsigned long)(uintptr_t)plugin_event);
    LeaveCriticalSection(&g_lock);
    return g_info;
}

BOOL WINAPI DllMain(HINSTANCE instance, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason == DLL_PROCESS_ATTACH) {
        g_module = instance;
        InitializeCriticalSection(&g_lock);
        g_lock_ready = TRUE;
        DisableThreadLibraryCalls(instance);
    } else if (reason == DLL_PROCESS_DETACH && g_lock_ready) {
        /* 加载器锁内不能等待。正常清理由禁用和卸载回调同步完成。 */
        close_handle(&g_stdin_write);
        close_handle(&g_stdout_read);
        close_handle(&g_process);
        heap_free(g_info);
        g_info = NULL;
        DeleteCriticalSection(&g_lock);
        g_lock_ready = FALSE;
    }
    return TRUE;
}
