/* GUI-style host has no stderr. Startup and callback failures must persist. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

typedef const char *(__stdcall *LoadFn)(uint32_t, uint32_t);
typedef void (__stdcall *LifecycleFn)(void);
static const char *__stdcall fake_core(const char *command) { (void)command; return ""; }

static uintptr_t pointer_field(const char *json, const char *key) {
    char pattern[80];
    const char *p;
    snprintf(pattern, sizeof(pattern), "\"%s\":", key);
    p = strstr(json, pattern);
    return p ? (uintptr_t)strtoul(p + strlen(pattern), NULL, 10) : 0;
}

static char *read_log(const wchar_t *path) {
    FILE *f = _wfopen(path, L"rb");
    char *data;
    long size;
    if (!f) return NULL;
    if (fseek(f, 0, SEEK_END) || (size = ftell(f)) < 0 || size > 1024 * 1024) { fclose(f); return NULL; }
    rewind(f);
    data = calloc((size_t)size + 1, 1);
    if (data) fread(data, 1, (size_t)size, f);
    fclose(f);
    return data;
}

#define CHECK(c, message) do { if (!(c)) { printf("FAIL diagnostic host: %s (win32=%lu)\n", message, GetLastError()); return 1; } } while (0)

int wmain(int argc, wchar_t **argv) {
    wchar_t root[2048], bridge_path[2300], stderr_path[2300], *slash;
    HMODULE dll;
    LoadFn load;
    LifecycleFn setting, disable;
    const char *info;
    char *bridge, *worker_stderr;
    ULONGLONG start;
    CHECK(argc == 3, "DLL path and diagnostic mode required");
    CHECK(GetModuleFileNameW(NULL, root, sizeof(root) / sizeof(root[0])), "host executable directory");
    slash = wcsrchr(root, L'\\'); CHECK(slash, "host directory separator"); *slash = 0;
    swprintf(bridge_path, sizeof(bridge_path) / sizeof(bridge_path[0]), L"%ls\\plugin_data\\BeeNT_Bridge_Test\\bridge.log", root);
    swprintf(stderr_path, sizeof(stderr_path) / sizeof(stderr_path[0]), L"%ls\\plugin_data\\BeeNT_Bridge_Test\\worker-stderr.log", root);
    DeleteFileW(bridge_path); DeleteFileW(stderr_path);
    CHECK(SetEnvironmentVariableW(L"BEENT_DIAGNOSTIC_MODE", argv[2]), "configure Worker failure");
    CHECK(SetStdHandle(STD_ERROR_HANDLE, NULL), "GUI host has no stderr handle");
    dll = LoadLibraryW(argv[1]); CHECK(dll, "LoadLibrary diagnostic DLL");
    load = (LoadFn)(uintptr_t)GetProcAddress(dll, "botPluginLoad"); CHECK(load, "botPluginLoad");
    info = load((uint32_t)(uintptr_t)fake_core, 19); CHECK(info && *info, "metadata available before Worker starts");
    setting = (LifecycleFn)pointer_field(info, "setting");
    disable = (LifecycleFn)pointer_field(info, "disable");
    CHECK(setting && disable, "setting and disable callbacks");
    start = GetTickCount64();
    setting(); /* No enable callback: settings must start and diagnose independently. */
    CHECK(GetTickCount64() - start < 10000, "failed settings request returns within bounded time");
    disable();
    CHECK(FreeLibrary(dll), "disable and unload failing Worker");
    bridge = read_log(bridge_path);
    CHECK(bridge && *bridge, "persistent bridge.log exists with GUI stderr absent");
    printf("bridge.log:\n%s\n", bridge);
    if (wcscmp(argv[2], L"stderr-exit") == 0) {
        worker_stderr = read_log(stderr_path);
        CHECK(worker_stderr && strstr(worker_stderr, "BEENT_DIAGNOSTIC_STDERR_MARKER"), "worker-stderr.log captures startup stderr marker");
        free(worker_stderr);
        CHECK(strstr(bridge, "stage=worker_exit win32=42"), "bridge.log records unexpected Worker exit code 42");
        CHECK(strstr(bridge, "exit_code"), "bridge.log identifies process exit detail");
    } else if (wcscmp(argv[2], L"callback-error") == 0) {
        CHECK(strstr(bridge, "BEENT_DIAGNOSTIC_CALLBACK_ERROR"), "bridge.log preserves event_result.error");
        CHECK(strstr(bridge, "stage=callback_error win32=0"), "bridge.log identifies callback failure stage");
        CHECK(strstr(bridge, "settings"), "callback error identifies settings event");
    } else if (wcscmp(argv[2], L"invalid-resource") == 0) {
        CHECK(strstr(bridge, "stage=CreateProcessW win32=193") || strstr(bridge, "stage=CreateProcessW win32=216"), "bridge.log records CreateProcess invalid-image error 193 or 216");
    } else CHECK(0, "unknown diagnostic test mode");
    free(bridge);
    puts("PASS: GUI host setting without enable persists diagnostic stage and failure detail");
    return 0;
}
