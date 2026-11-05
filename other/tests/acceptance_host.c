/* Acceptance commands exercise the final PE32 DLL and its embedded Go worker.
 * Host callback input and fake core output deliberately use CP936 bytes. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef const char *(__stdcall *LoadFn)(uint32_t, uint32_t);
typedef void (__stdcall *LifecycleFn)(void);
typedef int (__stdcall *PrivateFn)(const char *, const char *, const char *);
typedef int (__stdcall *GroupFn)(const char *, const char *, const char *, const char *);

#define SEP "#|&BEExNT&|#"
#define MAX_CALLS 512
#define MAX_ARGS 10
#define TEXT_CAPACITY 8192
typedef struct CoreCall {
    int opcode, index, argc;
    char args[MAX_ARGS][TEXT_CAPACITY];
    char response[256];
    ULONGLONG tick;
} CoreCall;

static CoreCall g_calls[MAX_CALLS];
static int g_call_count, g_invalid;
static LONG g_next_message = 700000;
static CRITICAL_SECTION g_lock;
static HANDLE g_changed;

static void copy_text(char *target, size_t capacity, const char *source, size_t length) {
    if (length >= capacity) length = capacity - 1;
    memcpy(target, source, length);
    target[length] = 0;
}

static const char *__stdcall fake_core(const char *command) {
    const char *field = command, *next;
    char token[TEXT_CAPACITY];
    int count = 0;
    CoreCall *call;
    EnterCriticalSection(&g_lock);
    if (g_call_count == MAX_CALLS) {
        g_invalid++;
        LeaveCriticalSection(&g_lock);
        return "ERROR acceptance host: too many API calls";
    }
    call = &g_calls[g_call_count++];
    call->tick = GetTickCount64();
    do {
        next = strstr(field, SEP);
        copy_text(token, sizeof(token), field, next ? (size_t)(next - field) : strlen(field));
        if (count == 0) call->opcode = atoi(token);
        else if (count == 1) call->index = atoi(token);
        else if (count - 2 < MAX_ARGS) strcpy(call->args[count - 2], token);
        else g_invalid++;
        count++;
        if (next) field = next + strlen(SEP);
    } while (next);
    call->argc = count - 2;
    if (call->index != 23) g_invalid++;
    switch (call->opcode) {
    case 1:
        strcpy(call->response, "{\"user_id\":999,\"nickname\":\"robot\"}");
        if (call->argc != 0) g_invalid++;
        break;
    case 2:
        if (call->argc != 1) g_invalid++;
        break;
    case 73:
        strcpy(call->response, "BeeNT-test");
        if (call->argc != 0) g_invalid++;
        break;
    case 74:
        strcpy(call->response, "test");
        if (call->argc != 0) g_invalid++;
        break;
    case 75:
        strcpy(call->response, "999");
        if (call->argc != 0) g_invalid++;
        break;
    case 3: case 4: case 82: case 83:
        snprintf(call->response, sizeof(call->response), "%ld", InterlockedIncrement(&g_next_message));
        if (call->argc != ((call->opcode == 3 || call->opcode == 4) ? 2 : 3)) g_invalid++;
        break;
    case 42:
        if (call->argc != 1) g_invalid++;
        break;
    default:
        snprintf(call->response, sizeof(call->response), "ERROR acceptance host: unsupported opcode %d", call->opcode);
        g_invalid++;
        break;
    }
    SetEvent(g_changed);
    LeaveCriticalSection(&g_lock);
    return call->response;
}

static uintptr_t pointer_field(const char *json, const char *key) {
    char pattern[80];
    const char *value;
    snprintf(pattern, sizeof(pattern), "\"%s\":", key);
    value = strstr(json, pattern);
    return value ? (uintptr_t)strtoul(value + strlen(pattern), NULL, 10) : 0;
}

static int call_count(void) {
    int count;
    EnterCriticalSection(&g_lock);
    count = g_call_count;
    LeaveCriticalSection(&g_lock);
    return count;
}

/* Each successful API result has stable storage for the entire host lifetime. */
static int find_call(int from, int opcode, const char *first, const char *second, CoreCall *found) {
    int i, result = 0;
    EnterCriticalSection(&g_lock);
    for (i = from; i < g_call_count; i++) {
        CoreCall *call = &g_calls[i];
        if (call->opcode != opcode) continue;
        if (first && strcmp(call->args[0], first) != 0) continue;
        if (second && strcmp(call->args[1], second) != 0) continue;
        if (found) *found = *call;
        result = 1;
        break;
    }
    LeaveCriticalSection(&g_lock);
    return result;
}

static int wait_call(int from, int opcode, const char *first, const char *second, DWORD timeout, CoreCall *found) {
    ULONGLONG deadline = GetTickCount64() + timeout;
    for (;;) {
        ULONGLONG now;
        ResetEvent(g_changed);
        if (find_call(from, opcode, first, second, found)) return 1;
        now = GetTickCount64();
        if (now >= deadline) return 0;
        WaitForSingleObject(g_changed, (DWORD)(deadline - now));
    }
}

static int send_count(int from) {
    int i, count = 0;
    EnterCriticalSection(&g_lock);
    for (i = from; i < g_call_count; i++) {
        int op = g_calls[i].opcode;
        if (op == 3 || op == 4 || op == 82 || op == 83 || op == 42) count++;
    }
    LeaveCriticalSection(&g_lock);
    return count;
}

static void dump_calls(int from) {
    int i, a;
    EnterCriticalSection(&g_lock);
    for (i = from; i < g_call_count; i++) {
        CoreCall *call = &g_calls[i];
        fprintf(stderr, "  API %d index=%d", call->opcode, call->index);
        for (a = 0; a < call->argc && a < MAX_ARGS; a++) fprintf(stderr, " arg%d=[%s]", a + 1, call->args[a]);
        fprintf(stderr, " -> [%s]\n", call->response);
    }
    LeaveCriticalSection(&g_lock);
}

#define CHECK(condition, message) do { if (!(condition)) { \
    fprintf(stderr, "FAIL acceptance host: %s (round=%d win32=%lu)\n", message, round + 1, GetLastError()); \
    dump_calls(round_start); return 1; \
} } while (0)

int wmain(int argc, wchar_t **argv) {
    int round = 0, round_start = 0;
    char expected_name[512];
    static const char echo_command[] = "/beent echo \xd6\xd0\xce\xc4\nsecond line \\uD83D\\uDE00";
    static const char echo_response[] = "\xd6\xd0\xce\xc4\nsecond line \\uD83D\\uDE00";
    InitializeCriticalSection(&g_lock);
    g_changed = CreateEventW(NULL, TRUE, FALSE, NULL);
    CHECK(argc == 2 || argc == 3, "DLL path argument required");
    CHECK(WideCharToMultiByte(936, 0, argc == 3 ? argv[2] : L"BeeNT Go \u9a8c\u6536\u63d2\u4ef6", -1, expected_name, sizeof(expected_name), NULL, NULL) > 0, "expected name CP936 conversion");
    CHECK(g_changed != NULL, "create API event");
    for (round = 0; round < 2; round++) {
        HMODULE dll;
        LoadFn load;
        LifecycleFn enable, disable;
        PrivateFn private_message;
        GroupFn group;
        const char *info;
        int from;
        CoreCall sent, recalled, background;
        ULONGLONG returned_at;
        round_start = call_count();
        dll = LoadLibraryW(argv[1]);
        CHECK(dll != NULL, "LoadLibrary final acceptance DLL");
        load = (LoadFn)(uintptr_t)GetProcAddress(dll, "botPluginLoad");
        CHECK(load != NULL, "undecorated botPluginLoad export");
        info = load((uint32_t)(uintptr_t)fake_core, 23);
        CHECK(info && strstr(info, expected_name), "self-test plugin metadata");
        enable = (LifecycleFn)pointer_field(info, "enable");
        disable = (LifecycleFn)pointer_field(info, "disable");
        private_message = (PrivateFn)pointer_field(info, "private");
        group = (GroupFn)pointer_field(info, "group");
        CHECK(enable && disable && private_message && group, "numeric stdcall callbacks");
        enable();
        from = call_count();
        CHECK(private_message("111", "/beent ping", "-101") == 0, "private command continues chain");
        CHECK(wait_call(from, 3, "111", "PONG", 2000, NULL), "private ping sends PONG through friend API");
        from = call_count();
        CHECK(group("222", "333", echo_command, "-102") == 0, "group echo continues chain");
        CHECK(wait_call(from, 4, "222", echo_response, 2000, NULL), "group echo preserves GBK Chinese and newline");
        from = call_count();
        CHECK(group("222", "333", "/beent quote", "-103") == 0, "quote continues chain");
        CHECK(wait_call(from, 83, "222", "-103", 2000, &sent), "group quote references incoming message ID");
        CHECK(sent.argc == 3 && sent.args[2][0], "quote has reply text");
        from = call_count();
        CHECK(private_message("111", "/beent quote", "-104") == 0, "private quote continues chain");
        CHECK(wait_call(from, 82, "111", "-104", 2000, NULL), "private quote references incoming message ID");
        from = call_count();
        CHECK(group("222", "333", "/beent recall", "-105") == 0, "recall continues chain");
        CHECK(wait_call(from, 42, NULL, NULL, 2000, &recalled), "recall invokes API");
        CHECK(strcmp(recalled.args[0], "-105") != 0, "recall never deletes user's incoming command");
        CHECK(strcmp(recalled.args[0], sent.response) == 0, "recall deletes latest plugin quote instead of an older reply");
        {
            int i, own_id = 0;
            EnterCriticalSection(&g_lock);
            for (i = round_start; i < g_call_count; i++) {
                int op = g_calls[i].opcode;
                if ((op == 4 || op == 83) && strcmp(g_calls[i].args[0], "222") == 0 && strcmp(g_calls[i].response, recalled.args[0]) == 0) own_id = 1;
            }
            LeaveCriticalSection(&g_lock);
            CHECK(own_id, "recall ID came from a successful plugin send");
        }
        from = call_count();
        CHECK(private_message("111", "/beent info", "-106") == 0, "info continues chain");
        CHECK(find_call(from, 73, NULL, NULL, NULL) && find_call(from, 74, NULL, NULL, NULL) && find_call(from, 75, NULL, NULL, NULL) && find_call(from, 1, NULL, NULL, NULL), "info uses all four framework and robot APIs");
        CHECK(find_call(from, 3, "111", NULL, &sent), "info replies to requesting friend");
        CHECK(strstr(sent.args[1], "BeeNT-test") && strstr(sent.args[1], "test") && strstr(sent.args[1], "999") && strstr(sent.args[1], "robot"), "info reply contains API results");
        from = call_count();
        CHECK(group("222", "333", "/beent id", "-107") == 0, "id continues chain");
        CHECK(find_call(from, 4, "222", NULL, &sent), "id replies to current group");
        CHECK(strstr(sent.args[1], "222") && strstr(sent.args[1], "333") && strstr(sent.args[1], "-107"), "id preserves group sender and signed message ID");
        from = call_count();
        CHECK(private_message("111", "/beent id", "-1071") == 0, "private id continues chain");
        CHECK(find_call(from, 3, "111", NULL, &sent), "private id replies to current friend");
        CHECK(strstr(sent.args[1], "111") && strstr(sent.args[1], "-1071"), "private id preserves source and signed message ID");
        from = call_count();
        CHECK(group("222", "333", "/beent bg", "-108") == 0, "background command continues chain");
        returned_at = GetTickCount64();
        CHECK(wait_call(from, 4, "222", "BG DONE", 5000, &background), "background reply arrives without another host callback");
        CHECK(background.tick >= returned_at + 1500, "background reply runs after callback return with two-second delay");
        CHECK(background.args[1][0], "background reply has content");
        CHECK(group("222", "333", "/beent intercept", "-1081") == 1, "only explicit intercept command blocks other plugins");
        CHECK(group("222", "333", "/beent bg", "-109") == 0, "second background task starts before disable");
        from = call_count();
        returned_at = GetTickCount64();
        disable();
        CHECK(GetTickCount64() - returned_at < 1500, "disable cancels two-second background task promptly");
        CHECK(send_count(from) == 0, "no delayed reply is sent while disabling");
        CHECK(private_message("111", "/beent ping", "-110") == 0, "disabled private callback continues");
        CHECK(group("222", "333", "/beent ping", "-111") == 0, "disabled group callback continues");
        Sleep(2300);
        CHECK(send_count(from) == 0, "disable cancels background work and prevents subsequent replies");
        CHECK(g_invalid == 0, "core opcodes argument counts and plugin index are valid");
        CHECK(FreeLibrary(dll), "FreeLibrary after disable supports reload");
        fprintf(stderr, "PASS acceptance round %d: ping, GBK/Unicode echo, quote, safe recall, info, id, background, disable\n", round + 1);
    }
    CloseHandle(g_changed);
    DeleteCriticalSection(&g_lock);
    puts("PASS: final acceptance DLL and embedded Go worker, two LoadLibrary/disable/FreeLibrary rounds");
    return 0;
}
