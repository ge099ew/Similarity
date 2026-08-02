#include <stdio.h>
#include <time.h>

static inline int inc(int x) { return x + 1; }

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    int v = 0;
    for (int i = 0; i < 1000000; i++) v = inc(v);
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("result: %d  time: %.2fms\n", v, ms);
    return 0;
}
