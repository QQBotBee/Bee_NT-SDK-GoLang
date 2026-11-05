/* Smoke test for the production Go worker embedded in the final DLL. */
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
static volatile LONG g_calls,g_invalid,g_chinese;
static char g_worker_marker[512];
static volatile LONG g_marker_calls;
static const char *__stdcall fake_core(const char *command) {
    const char *prefix="2#|&BEExNT&|#13#|&BEExNT&|#";const unsigned char *p=(const unsigned char *)command;
    if(strncmp(command,prefix,strlen(prefix))!=0)InterlockedIncrement(&g_invalid);
    while(*p){if(*p>=0x80){InterlockedIncrement(&g_chinese);break;}p++;}
    if(g_worker_marker[0]&&strstr(command,g_worker_marker))InterlockedIncrement(&g_marker_calls);
    InterlockedIncrement(&g_calls);return "";
}
static uintptr_t pointer_field(const char *json,const char *key) {
    char pattern[80];const char *p;
    snprintf(pattern,sizeof(pattern),"\"%s\":",key);p=strstr(json,pattern);
    return p?(uintptr_t)strtoul(p+strlen(pattern),NULL,10):0;
}
#define CHECK(c,m) do{if(!(c)){fprintf(stderr,"FAIL production Go worker: %s (win32=%lu)\n",m,GetLastError());return 1;}}while(0)
int wmain(int argc,wchar_t **argv) {
    int iteration;LONG count=0;char chinese[]="\xd6\xd0\xce\xc4";
    CHECK(argc>=2 && argc<=4,"DLL path argument required");
    if(argc==4)CHECK(WideCharToMultiByte(936,0,argv[3],-1,g_worker_marker,sizeof(g_worker_marker),NULL,NULL)>0,"expected worker marker conversion");
    for(iteration=0;iteration<3;iteration++) {
        HMODULE dll=LoadLibraryW(argv[1]);LoadFn load;const char *info;
        LifecycleFn enable,disable,uninstall;PrivateFn private_message;GroupFn group;EventFn event;
        CHECK(dll,"LoadLibraryW");load=(LoadFn)(uintptr_t)GetProcAddress(dll,"botPluginLoad");CHECK(load,"export");
        info=load((uint32_t)(uintptr_t)fake_core,13);CHECK(info&&strstr(info,"\"desc\":"),"metadata");
        if(argc>=3) {
            char expected[512];
            CHECK(WideCharToMultiByte(936,0,argv[2],-1,expected,sizeof(expected),NULL,NULL)>0,"expected metadata name conversion");
            CHECK(strstr(info,expected)!=NULL,"alternate PluginName reached final DLL metadata");
        }
        enable=(LifecycleFn)pointer_field(info,"enable");disable=(LifecycleFn)pointer_field(info,"disable");uninstall=(LifecycleFn)pointer_field(info,"uninstall");
        private_message=(PrivateFn)pointer_field(info,"private");group=(GroupFn)pointer_field(info,"group");event=(EventFn)pointer_field(info,"event");
        CHECK(enable&&disable&&uninstall&&private_message&&group&&event,"callbacks");CHECK(g_calls==count,"load has no Go business side effects");
        fprintf(stderr,"Go host iteration %d: enable\n",iteration+1);
        enable();count+=2;CHECK(g_calls==count,"initialize and enable API log through production worker");
        CHECK(private_message("123",chinese,"-10")==0,"private callback continues");
        CHECK(group("456","123",chinese,"-11")==0,"group callback continues");
        CHECK(event("event","456",chinese,"123",chinese,"789",chinese,"flag")==0,"eight argument event continues");
        disable();count++;CHECK(g_calls==count,"disable invokes Go callback before termination");
        if(iteration==2){uninstall();count+=3;CHECK(g_calls==count,"uninstall starts worker and invokes final cleanup");}
        CHECK(FreeLibrary(dll),"FreeLibrary/reload");CHECK(g_invalid==0,"opcode and plugin index preserved");CHECK(g_chinese==count,"all UTF-8 logs converted to GBK");
    }
    if(g_worker_marker[0])CHECK(g_marker_calls>=3,"alternate business code reached embedded Go worker");
    puts("PASS: final DLL + production Go worker, initialize/enable/API/three callbacks/disable/uninstall/reload");return 0;
}
