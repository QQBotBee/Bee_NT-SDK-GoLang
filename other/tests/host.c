/* Real x86 LoadLibrary/stdcall integration test. Run via run.ps1. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
typedef const char *(__stdcall *LoadFn)(uint32_t,uint32_t);
typedef void (__stdcall *LifecycleFn)(void);
typedef int (__stdcall *PrivateFn)(const char*,const char*,const char*);
typedef int (__stdcall *GroupFn)(const char*,const char*,const char*,const char*);
typedef int (__stdcall *EventFn)(const char*,const char*,const char*,const char*,const char*,const char*,const char*,const char*);
static HANDLE g_called,g_release;
static volatile LONG g_calls,g_invalid;
static int g_block_core;
static char g_chinese[]="\xd6\xd0\xce\xc4";
static DWORD WINAPI group_thread(void *context) {
    GroupFn callback=(GroupFn)(uintptr_t)context;
    return (DWORD)callback("456","123",g_chinese,"m");
}
static const char *__stdcall fake_core(const char *command) {
    if(strncmp(command,"2#|&BEExNT&|#7#|&BEExNT&|#",24)!=0 || !strstr(command,g_chinese))InterlockedIncrement(&g_invalid);
    if(InterlockedIncrement(&g_calls)%2==0){SetEvent(g_called);if(g_block_core)WaitForSingleObject(g_release,INFINITE);}
    return g_chinese;
}
static uintptr_t pointer_field(const char *json,const char *key) {
    char pattern[80];const char *p;
    snprintf(pattern,sizeof(pattern),"\"%s\":",key);p=strstr(json,pattern);
    return p?(uintptr_t)strtoul(p+strlen(pattern),NULL,10):0;
}
#define CHECK(c,m) do{if(!(c)){fprintf(stderr,"FAIL: %s (win32=%lu)\n",m,GetLastError());return 1;}}while(0)
int main(int argc,char **argv) {
    int iteration; LONG expected_calls=0;g_called=CreateEventW(NULL,TRUE,FALSE,NULL);g_release=CreateEventW(NULL,TRUE,FALSE,NULL);
    CHECK(argc>=2,"DLL argument required");
    if(argc==3) {
        HMODULE dll=LoadLibraryA(argv[1]);LoadFn load;const char *info;LifecycleFn enable,disable,uninstall;GroupFn group;ULONGLONG start;
        CHECK(dll!=NULL,"special case LoadLibrary");
        load=(LoadFn)(uintptr_t)GetProcAddress(dll,"botPluginLoad");CHECK(load,"special case load export");
        info=load((uint32_t)(uintptr_t)fake_core,7);CHECK(info&&*info,"special case metadata");
        enable=(LifecycleFn)pointer_field(info,"enable");disable=(LifecycleFn)pointer_field(info,"disable");uninstall=(LifecycleFn)pointer_field(info,"uninstall");group=(GroupFn)pointer_field(info,"group");
        if(strcmp(argv[2],"blocked-core")==0)g_block_core=1;
        else CHECK(SetEnvironmentVariableA("BEENT_BRIDGE_TEST_STALL","1"),"configure stalled peer");
        enable();CHECK(WaitForSingleObject(g_called,5000)==WAIT_OBJECT_0,"special case background request");
        start=GetTickCount64();
        if(strncmp(argv[2],"concurrent-",11)==0) {
            HANDLE threads[2];DWORD first,second;
            threads[0]=CreateThread(NULL,0,group_thread,(void *)(uintptr_t)group,0,NULL);
            Sleep(25);threads[1]=CreateThread(NULL,0,group_thread,(void *)(uintptr_t)group,0,NULL);Sleep(25);
            CHECK(threads[0]&&threads[1],"concurrent callback threads");
            if(strcmp(argv[2],"concurrent-uninstall")==0)uninstall();else disable();
            CHECK(GetTickCount64()-start<5000,"concurrent lifecycle cancellation bounded");
            CHECK(FreeLibrary(dll),"immediate concurrent FreeLibrary");
            CHECK(WaitForMultipleObjects(2,threads,TRUE,5000)==WAIT_OBJECT_0,"in-flight and waiting callbacks finish safely");
            CHECK(GetExitCodeThread(threads[0],&first)&&GetExitCodeThread(threads[1],&second)&&first==0&&second==0,"cancelled callbacks continue chain");
            CHECK(GetModuleHandleA(argv[1])==NULL,"concurrent unload releases module guards");
            CloseHandle(threads[0]);CloseHandle(threads[1]);CloseHandle(g_called);CloseHandle(g_release);
            puts("PASS: concurrent in-flight + queued callbacks cancelled/drained before lifecycle unload");return 0;
        }
        if(!g_block_core)CHECK(group("456","123",g_chinese,"m")==0,"timed out callback continues chain");
        disable();CHECK(GetTickCount64()-start<5000,"bounded stalled shutdown");
        CHECK(FreeLibrary(dll),"special case FreeLibrary");
        if(g_block_core) {
            CHECK(GetModuleHandleA(argv[1])!=NULL,"reader pins DLL while core API is blocked");
            SetEvent(g_release);start=GetTickCount64();
            while(GetModuleHandleA(argv[1])&&GetTickCount64()-start<5000)Sleep(10);
            CHECK(GetModuleHandleA(argv[1])==NULL,"last reader reference releases DLL after API returns");
        }
        puts(g_block_core?"PASS: blocked core API, bounded disable, DLL lifetime":"PASS: stalled worker, bounded event timeout and disable");
        CloseHandle(g_called);CloseHandle(g_release);return 0;
    }
    for(iteration=0;iteration<3;iteration++) {
        HMODULE dll=LoadLibraryA(argv[1]);const char *info;LoadFn load;
        LifecycleFn enable,disable,uninstall,setting;
        PrivateFn private_message;GroupFn group;EventFn event;
        CHECK(dll!=NULL,"LoadLibrary");
        load=(LoadFn)(uintptr_t)GetProcAddress(dll,"botPluginLoad");
        CHECK(load!=NULL,"undecorated botPluginLoad export");
        info=load((uint32_t)(uintptr_t)fake_core,7);
        CHECK(info && strstr(info,"\"desc\":"),"BeeNT metadata fields");
        enable=(LifecycleFn)pointer_field(info,"enable");disable=(LifecycleFn)pointer_field(info,"disable");
        uninstall=(LifecycleFn)pointer_field(info,"uninstall");setting=(LifecycleFn)pointer_field(info,"setting");
        private_message=(PrivateFn)pointer_field(info,"private");group=(GroupFn)pointer_field(info,"group");event=(EventFn)pointer_field(info,"event");
        CHECK(enable&&disable&&uninstall&&setting&&private_message&&group&&event,"all seven numeric callbacks");
        CHECK(g_calls==expected_calls,"load must not launch worker or call API");
        ResetEvent(g_called);enable();
        CHECK(WaitForSingleObject(g_called,5000)==WAIT_OBJECT_0,"background API without host callback");
        expected_calls+=2;
        CHECK(private_message("123",g_chinese,"m")==1,"private callback");
        CHECK(group("456","123",g_chinese,"m")==1,"GBK group callback and intercept return");
        CHECK(event("event","456",g_chinese,"123",g_chinese,"789",g_chinese,"flag")==1,"eight-argument event callback");
        disable();
        /* Reload calls disable then FreeLibrary without uninstall. */
        if(iteration==2){uninstall();expected_calls++;}
        CHECK(FreeLibrary(dll),"FreeLibrary");
        CHECK(g_invalid==0,"UTF-8 command converted to GBK, exact index preserved");
        CHECK(g_calls==expected_calls,"exact lifecycle API call count");
    }
    CloseHandle(g_called);CloseHandle(g_release);puts("PASS: BeeNT metadata, stdcall callbacks, GBK API, background IPC, disable/reload");return 0;
}
