/* Deliberately failing Worker for native diagnostic integration tests. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    char line[16384];
    const char *mode = getenv("BEENT_DIAGNOSTIC_MODE");
    if (mode && strcmp(mode, "stderr-exit") == 0) {
        fputs("BEENT_DIAGNOSTIC_STDERR_MARKER\n", stderr);
        fflush(stderr);
        return 42;
    }
    while (fgets(line, sizeof(line), stdin)) {
        char *id = strstr(line, "\"id\":\"");
        char *end;
        if (!id) return 43;
        id += 6;
        end = strchr(id, '"');
        if (!end) return 44;
        *end = 0;
        if (mode && strcmp(mode, "callback-error") == 0 && strstr(end + 1, "\"event\":\"settings\"")) {
            printf("{\"type\":\"event_result\",\"id\":\"%s\",\"error\":\"BEENT_DIAGNOSTIC_CALLBACK_ERROR\"}\n", id);
        } else {
            printf("{\"type\":\"event_result\",\"id\":\"%s\",\"result\":0}\n", id);
        }
        fflush(stdout);
        if (strstr(end + 1, "\"event\":\"stop\"")) break;
    }
    return 0;
}
