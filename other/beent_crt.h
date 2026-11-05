#ifndef BEENT_MINIMAL_CRT_H
#define BEENT_MINIMAL_CRT_H
/* Private freestanding helpers for beent_bridge.c. Include this header first
 * (-include other/beent_crt.h), then link -nostdlib -lkernel32. This is not a
 * general libc: format support is intentionally limited to the bridge's uses. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stddef.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

/* Volatile stores prevent loop recognition from recursively calling libc.
 * Public spellings also satisfy compiler-emitted memory helper references. */
__attribute__((noinline)) void *memcpy(void *destination,const void *source,size_t size) {
    volatile unsigned char *d=destination;const volatile unsigned char *s=source;
    size_t i;for(i=0;i<size;i++)d[i]=s[i];return destination;
}
__attribute__((noinline)) void *memset(void *destination,int value,size_t size) {
    volatile unsigned char *d=destination;size_t i;for(i=0;i<size;i++)d[i]=(unsigned char)value;return destination;
}
__attribute__((noinline)) void *memmove(void *destination,const void *source,size_t size) {
    volatile unsigned char *d=destination;const volatile unsigned char *s=source;size_t i;
    if((uintptr_t)d<(uintptr_t)s){for(i=0;i<size;i++)d[i]=s[i];}else{for(i=size;i>0;i--)d[i-1]=s[i-1];}return destination;
}

static size_t beent_strlen(const char *s){const char *p=s;while(*p)p++;return (size_t)(p-s);}
static size_t beent_wcslen(const WCHAR *s){const WCHAR *p=s;while(*p)p++;return (size_t)(p-s);}
static int beent_strcmp(const char *a,const char *b){while(*a&&*a==*b){a++;b++;}return (unsigned char)*a-(unsigned char)*b;}
static char *beent_strchr(const char *s,int c){do{if((unsigned char)*s==(unsigned char)c)return (char *)s;}while(*s++);return NULL;}
static WCHAR *beent_wcsrchr(const WCHAR *s,WCHAR c){const WCHAR *found=NULL;do{if(*s==c)found=s;}while(*s++);return (WCHAR *)found;}
static char *beent_strcpy(char *d,const char *s){char *result=d;while((*d++=*s++)!=0){}return result;}
static char *beent_strstr(const char *haystack,const char *needle){const char *p,*n;if(!*needle)return (char *)haystack;while(*haystack){p=haystack;n=needle;while(*p&&*p==*n){p++;n++;}if(!*n)return (char *)haystack;haystack++;}return NULL;}
static size_t beent_strspn(const char *s,const char *accept){size_t count=0;while(s[count]&&beent_strchr(accept,(unsigned char)s[count]))count++;return count;}
static void *beent_memchr(const void *s,int value,size_t size){const unsigned char *p=s;size_t i;for(i=0;i<size;i++)if(p[i]==(unsigned char)value)return (void *)(p+i);return NULL;}

static unsigned long beent_strtoul(const char *text,char **end,int base){
    const char *start=text,*digits;unsigned long value=0;int negative=0,overflow=0;
    while(*text==' '||*text=='\t'||*text=='\r'||*text=='\n'||*text=='\v'||*text=='\f')text++;
    if(*text=='-'||*text=='+'){negative=*text=='-';text++;}
    if((base==0||base==16)&&text[0]=='0'&&(text[1]=='x'||text[1]=='X')){base=16;text+=2;}
    if(base==0)base=*text=='0'?8:10;digits=text;
    while(*text){unsigned int digit=*text>='0'&&*text<='9'?(unsigned int)(*text-'0'):*text>='a'&&*text<='z'?(unsigned int)(*text-'a'+10):*text>='A'&&*text<='Z'?(unsigned int)(*text-'A'+10):100;
        if(digit>=(unsigned int)base)break;
        if(value>(0xfffffffful-digit)/(unsigned long)base)overflow=1;else if(!overflow)value=value*(unsigned long)base+digit;text++;
    }
    if(end)*end=(char *)(text==digits?start:text);if(overflow)return 0xfffffffful;return negative?0ul-value:value;
}
static int beent_atoi(const char *s){int negative=0;unsigned int value=0;while(*s==' '||*s=='\t'||*s=='\r'||*s=='\n')s++;if(*s=='-'||*s=='+'){negative=*s=='-';s++;}while(*s>='0'&&*s<='9'){value=value*10u+(unsigned int)(*s-'0');s++;}return negative?(int)(0u-value):(int)value;}

typedef struct BeentWriter {void *buffer;size_t capacity,length;int wide;} BeentWriter;
static void beent_put(BeentWriter *writer,unsigned int c){if(writer->capacity&&writer->length<writer->capacity-1){if(writer->wide)((WCHAR *)writer->buffer)[writer->length]=(WCHAR)c;else ((char *)writer->buffer)[writer->length]=(char)c;}writer->length++;}
static void beent_end(BeentWriter *writer){size_t at=writer->length<writer->capacity?writer->length:writer->capacity-1;if(writer->capacity){if(writer->wide)((WCHAR *)writer->buffer)[at]=0;else ((char *)writer->buffer)[at]=0;}}
static unsigned int beent_format_at(const void *format,size_t index,int wide){return wide?((const WCHAR *)format)[index]:(unsigned char)((const char *)format)[index];}
static int beent_vformat(void *buffer,size_t capacity,const void *format,int wide,va_list args){
    BeentWriter writer={buffer,capacity,0,wide};size_t at=0;unsigned int c;
    while((c=beent_format_at(format,at++,wide))!=0){
        unsigned int width=0;int zero=0,precision=-1,is_long=0;
        if(c!='%'){beent_put(&writer,c);continue;}
        c=beent_format_at(format,at++,wide);if(c=='%'){beent_put(&writer,'%');continue;}
        if(c=='0'){zero=1;c=beent_format_at(format,at++,wide);}
        while(c>='0'&&c<='9'){width=width*10u+c-'0';c=beent_format_at(format,at++,wide);}
        if(c=='.'){precision=0;c=beent_format_at(format,at++,wide);if(c=='*'){precision=va_arg(args,int);c=beent_format_at(format,at++,wide);}else{while(c>='0'&&c<='9'){precision=precision*10+(int)c-'0';c=beent_format_at(format,at++,wide);}}}
        if(c=='l'){is_long=1;c=beent_format_at(format,at++,wide);}
        if(c=='s'){
            const void *s=va_arg(args,const void *);size_t n=0,i;int string_wide=wide&&is_long;
            if(!s)s=string_wide?(const void *)L"(null)":(const void *)"(null)";
            while((precision<0||n<(size_t)precision)&&beent_format_at(s,n,string_wide))n++;
            for(i=n;i<(size_t)width;i++)beent_put(&writer,' ');for(i=0;i<n;i++)beent_put(&writer,beent_format_at(s,i,string_wide));
        }else if(c=='u'||c=='d'){
            unsigned long value;int negative=0;char digits[16];unsigned int n=0,i;
            if(c=='d'){long signed_value=is_long?va_arg(args,long):(long)va_arg(args,int);negative=signed_value<0;value=negative?0ul-(unsigned long)signed_value:(unsigned long)signed_value;}
            else value=is_long?va_arg(args,unsigned long):(unsigned long)va_arg(args,unsigned int);
            do{digits[n++]=(char)('0'+value%10ul);value/=10ul;}while(value);
            if(!zero){for(i=n+(unsigned int)negative;i<width;i++)beent_put(&writer,' ');}if(negative)beent_put(&writer,'-');if(zero){for(i=n+(unsigned int)negative;i<width;i++)beent_put(&writer,'0');}
            while(n)beent_put(&writer,(unsigned int)digits[--n]);
        }else{beent_put(&writer,'%');if(c)beent_put(&writer,c);else break;}
    }
    beent_end(&writer);return (int)writer.length;
}
static int beent_snprintf(char *buffer,size_t capacity,const char *format,...){int n;va_list args;va_start(args,format);n=beent_vformat(buffer,capacity,format,0,args);va_end(args);return n;}
static int beent_sprintf(char *buffer,const char *format,...){int n;va_list args;va_start(args,format);n=beent_vformat(buffer,(size_t)-1,format,0,args);va_end(args);return n;}
static int beent_swprintf(WCHAR *buffer,size_t capacity,const WCHAR *format,...){int n;va_list args;va_start(args,format);n=beent_vformat(buffer,capacity,format,1,args);va_end(args);return capacity&&(size_t)n<capacity?n:-1;}

#define strlen beent_strlen
#define wcslen beent_wcslen
#define strcmp beent_strcmp
#define strchr beent_strchr
#define wcsrchr beent_wcsrchr
#define strcpy beent_strcpy
#define strstr beent_strstr
#define strspn beent_strspn
#define strtoul beent_strtoul
#define atoi beent_atoi
#define memchr beent_memchr
#define snprintf beent_snprintf
#define sprintf beent_sprintf
#define swprintf beent_swprintf
#endif
