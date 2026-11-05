/* A deterministic protocol peer for testing the real DLL independently of plugin business code. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static char line[32768];
static void field(const char *input,const char *key,char *out,size_t capacity) {
    char pattern[80]; const char *p,*end; size_t n;
    snprintf(pattern,sizeof(pattern),"\"%s\":\"",key);
    p=strstr(input,pattern); if(!p){out[0]=0;return;} p+=strlen(pattern);
    end=strchr(p,'"'); if(!end){out[0]=0;return;} n=(size_t)(end-p);
    if(n>=capacity)n=capacity-1; memcpy(out,p,n);out[n]=0;
}
static int call_api(const char *id) {
    /* UTF-8: 2#|&BEExNT&|#7#|&BEExNT&|#中文 */
    printf("{\"type\":\"api_call\",\"id\":\"%s\",\"command_b64\":\"MiN8JkJFRXhOVCZ8IzcjfCZCRUV4TlQmfCPkuK3mloc=\"}\n",id);
    fflush(stdout);
    while(fgets(line,sizeof(line),stdin)) {
        char pending_id[80],event[80];int result=0;
        if(strstr(line,"\"type\":\"api_result\""))return strstr(line,"5Lit5paH") != NULL;
        field(line,"event",event,sizeof(event));field(line,"id",pending_id,sizeof(pending_id));
        if(strcmp(event,"private_message")==0 || strcmp(event,"common_event")==0)result=1;
        if(strcmp(event,"group_message")==0) {
            if(getenv("BEENT_BRIDGE_TEST_STALL"))Sleep(INFINITE);
            result=strstr(line,"1tDOxA==")?1:0;
        }
        printf("{\"type\":\"event_result\",\"id\":\"%s\",\"result\":%d}\n",pending_id,result);
    }
    return 0;
}
int main(void) {
    setvbuf(stdout,NULL,_IONBF,0);
    while(fgets(line,sizeof(line),stdin)) {
        char event[80],id[80];int result=0;
        field(line,"event",event,sizeof(event));field(line,"id",id,sizeof(id));
        if(strcmp(event,"group_message")==0) {
            if(getenv("BEENT_BRIDGE_TEST_STALL"))Sleep(INFINITE);
            result=strstr(line,"1tDOxA==")?1:0;
        }
        if(strcmp(event,"private_message")==0) result=1;
        if(strcmp(event,"common_event")==0) result=1;
        if(strcmp(event,"initialize")==0 && !call_api("init"))return 3;
        /* A stale response must never complete the pending callback. */
        printf("{\"type\":\"event_result\",\"id\":\"999999\",\"result\":1}\n");
        printf("{\"type\":\"event_result\",\"id\":\"%s\",\"result\":%d}\n",id,result);
        if(strcmp(event,"enable")==0) {
            Sleep(25);
            if(!call_api("background"))return 4;
        }
        if(strcmp(event,"stop")==0 || strcmp(event,"unload")==0)return 0;
    }
    return 0;
}
