/* Freestanding x86 regression probe: no CRT, no test-framework dependency. */
#include "../beent_crt.h"

static int failures;
extern DWORD test_alloca_size(DWORD size);

static void report(const char *message) {
    DWORD written;
    WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), message, (DWORD)strlen(message), &written, NULL);
}

#define CHECK(condition, message) do { if (!(condition)) { report("FAIL: " message "\r\n"); failures++; } } while (0)

static int wide_equal(const WCHAR *a, const WCHAR *b) {
    while (*a && *a == *b) { a++; b++; }
    return *a == *b;
}

static void formatting(void) {
    char text[512], small[5] = {'x','x','x','x','z'};
    WCHAR wide[256], short_wide[4] = {1,1,1,99};
    const char gbk[] = {(char)0xd6,(char)0xd0,(char)0xce,(char)0xc4,0};
    int n;
    n = snprintf(text, sizeof(text), "%04u-%02u-%02uT%02u:%02u:%02u.%03u pid=%lu tid=%lu stage=%s win32=%lu %.*s\r\n",
        2026u,11u,5u,1u,2u,3u,9u,4294967295ul,77ul,"callback_error",193ul,4,"error ignored");
    CHECK(strcmp(text,"2026-11-05T01:02:03.009 pid=4294967295 tid=77 stage=callback_error win32=193 erro\r\n")==0,"timestamp, uint32 and precision formatting");
    CHECK(n==(int)strlen(text),"snprintf length");
    CHECK(snprintf(small,4,"abcdef")==6,"snprintf truncated length");
    CHECK(strcmp(small,"abc")==0&&small[4]=='z',"snprintf bounds and terminator");
    CHECK(snprintf(NULL,0,"%s-%u","size",12u)==7,"snprintf zero capacity");
    sprintf(text,"%d %05d %% %s",(-2147483647-1),-12,"text");
    CHECK(strcmp(text,"-2147483648 -0012 % text")==0,"signed minimum and sign padding");
    snprintf(text,sizeof(text),"%s",gbk);
    CHECK(strlen(text)==4&&strcmp(text,gbk)==0,"GBK bytes remain unchanged");
    CHECK(swprintf(wide,256,L"%ls\\plugin_data\\%ls\\worker_%lu.exe",L"C:\\中文",L"插件",4294967295ul)>0,"wide path format result");
    CHECK(wide_equal(wide,L"C:\\中文\\plugin_data\\插件\\worker_4294967295.exe"),"wide Unicode path and uint32 format");
    CHECK(swprintf(short_wide,3,L"abcdef")==-1&&wide_equal(short_wide,L"ab")&&short_wide[3]==99,"wide truncated bounds");
}

static void strings_and_memory(void) {
    char data[32], *end;
    const char *sample="abcabc";
    size_t i;
    CHECK(strlen("")==0&&strlen("hello")==5&&wcslen(L"中文")==2,"string lengths");
    CHECK(strcmp("abc","abc")==0&&strcmp("abc","abd")<0&&strcmp("abd","abc")>0,"string comparisons");
    CHECK(strchr(sample,'b')==sample+1&&strchr(sample,0)==sample+6&&strchr(sample,'z')==NULL,"strchr");
    CHECK(strstr(sample,"bca")==sample+1&&strstr(sample,"")==sample&&strstr(sample,"zzz")==NULL,"strstr");
    CHECK(strspn("  \t12"," \t")==3,"strspn");
    CHECK(strtoul("  +4294967295x",&end,10)==4294967295ul&&*end=='x',"strtoul full uint32 and end");
    CHECK(strtoul("42949672960",&end,10)==4294967295ul&&*end==0,"strtoul overflow saturation");
    CHECK(strtoul("-1",NULL,10)==4294967295ul&&atoi(" -19 ")==-19,"signed integer parsing");
    strcpy(data,"abcdef");
    memmove(data+1,data,7);
    CHECK(strcmp(data,"aabcdef")==0,"memmove overlapping forward");
    memmove(data,data+1,7);
    CHECK(strcmp(data,"abcdef")==0,"memmove overlapping backward");
    memset(data,0x9a,sizeof(data));
    for(i=0;i<sizeof(data);i++)CHECK((unsigned char)data[i]==0x9a,"memset byte");
    memcpy(data,"hello",6);
    CHECK(strcmp(data,"hello")==0&&memchr(data,'l',5)==data+2&&memchr(data,'z',5)==NULL,"memcpy and memchr");
}

/* Volatile storage forces compiler stack allocations beyond a single guard
 * page. A fresh thread starts with a small committed stack, so jumping over
 * guard pages would fail here instead of merely reusing previously grown pages. */
__attribute__((noinline)) static DWORD large_stack(DWORD seed) {
    volatile unsigned char bytes[196608];
    DWORD i, checksum=0;
    for(i=0;i<sizeof(bytes);i++)bytes[i]=(unsigned char)(i+seed);
    for(i=0;i<sizeof(bytes);i++)checksum+=bytes[i];
    return checksum==25067520u ? seed : 0;
}

static DWORD WINAPI stack_thread(void *argument) {
    DWORD seed=(DWORD)(uintptr_t)argument;
    return large_stack(seed)==seed ? 0 : 1;
}

void test_start(void) {
    HANDLE thread;
    DWORD exit_code=99;
    formatting();
    strings_and_memory();
    CHECK(test_alloca_size(0),"alloca zero-byte ABI");
    CHECK(test_alloca_size(4095),"alloca page-minus-one ABI");
    CHECK(test_alloca_size(4096),"alloca exact-page ABI");
    CHECK(test_alloca_size(4097),"alloca page-plus-one ABI");
    CHECK(test_alloca_size(8192),"alloca two-page ABI");
    thread=CreateThread(NULL,1024*1024,stack_thread,(void *)(uintptr_t)37,STACK_SIZE_PARAM_IS_A_RESERVATION,NULL);
    CHECK(thread!=NULL,"stack probe thread creation");
    if(thread){
        CHECK(WaitForSingleObject(thread,10000)==WAIT_OBJECT_0,"stack probe thread completion");
        CHECK(GetExitCodeThread(thread,&exit_code)&&exit_code==0,"stack probe guard pages, contents and return address");
        CloseHandle(thread);
    }
    if(!failures)report("PASS: freestanding formatting, memory and x86 allocating guard-page stack probe\r\n");
    ExitProcess((UINT)failures);
}
