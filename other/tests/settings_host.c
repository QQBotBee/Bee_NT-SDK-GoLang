/* Verify the final DLL -> Worker -> real user32 settings window lifecycle. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
typedef const char *(__stdcall *LoadFn)(uint32_t,uint32_t);
typedef void (__stdcall *LifecycleFn)(void);
static const char *__stdcall fake_core(const char *command) {
    if(strncmp(command,"75#|&BEExNT&|#",13)==0)return "999";
    if(strncmp(command,"1#|&BEExNT&|#",12)==0)return "{\"user_id\":999,\"nickname\":\"robot\"}";
    return "";
}
static uintptr_t pointer_field(const char *json,const char *key) {
    char pattern[80];const char *p;
    snprintf(pattern,sizeof(pattern),"\"%s\":",key);p=strstr(json,pattern);
    return p?(uintptr_t)strtoul(p+strlen(pattern),NULL,10):0;
}
static HWND wait_window(const WCHAR *title) {
    ULONGLONG deadline=GetTickCount64()+10000;HWND window;
    do {
        window=FindWindowW(L"BeeNTGoSettingsWindow",title);
        if(window&&IsWindowVisible(window))return window;
        Sleep(10);
    }while(GetTickCount64()<deadline);
    return NULL;
}
static int wait_closed(HWND window) {
    ULONGLONG deadline=GetTickCount64()+5000;
    while(IsWindow(window)&&GetTickCount64()<deadline)Sleep(10);
    return !IsWindow(window);
}
#define CHECK(condition,message) do{if(!(condition)){fprintf(stderr,"FAIL settings host: %s (win32=%lu)\n",message,GetLastError());if(disable)disable();if(dll)FreeLibrary(dll);return 1;}}while(0)
int wmain(int argc,WCHAR **argv) {
    HMODULE dll=NULL;LifecycleFn setting=NULL,enable=NULL,disable=NULL;LoadFn load;const char *info;HWND first,second;WCHAR title[1024];
    CHECK(argc==3,"usage: settings_host.exe DLL-path PluginName");
    swprintf(title,sizeof(title)/sizeof(title[0]),L"%ls \x8bbe\x7f6e",argv[2]);
    CHECK(!FindWindowW(L"BeeNTGoSettingsWindow",title),"test window already exists; close it before running");
    dll=LoadLibraryW(argv[1]);CHECK(dll,"LoadLibraryW");
    load=(LoadFn)(uintptr_t)GetProcAddress(dll,"botPluginLoad");CHECK(load,"botPluginLoad export");
    info=load((uint32_t)(uintptr_t)fake_core,13);CHECK(info&&*info,"initialization metadata");
    setting=(LifecycleFn)pointer_field(info,"setting");enable=(LifecycleFn)pointer_field(info,"enable");disable=(LifecycleFn)pointer_field(info,"disable");
    CHECK(setting&&enable&&disable,"lifecycle callback pointers");
    setting();first=wait_window(title);CHECK(first,"settings callback before enable creates a visible real window");
    setting();second=wait_window(title);CHECK(second==first,"second settings callback reuses the same HWND");
    CHECK(PostMessageW(first,WM_CLOSE,0,0),"post close");CHECK(wait_closed(first),"user close destroys HWND");
    setting();second=wait_window(title);CHECK(second,"settings callback reopens closed window");
    enable();disable();CHECK(wait_closed(second),"disable destroys settings HWND");
    CHECK(FreeLibrary(dll),"FreeLibrary after disable");dll=NULL;
    puts("PASS: final DLL settings before enable, visible HWND, single instance, WM_CLOSE, reopen, disable cleanup");
    return 0;
}
