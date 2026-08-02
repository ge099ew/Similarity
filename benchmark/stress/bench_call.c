#include <stdio.h>
#include <time.h>

static int inc(int x) { return x + 1; }

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    int v = 0;
    for (int i = 0; i < 1000000; i++) v = inc(v);
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %d  time: %.2fms\n", v, ms);
    return 0;
}
