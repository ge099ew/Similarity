#include <stdio.h>
#include <time.h>

static long bubble_sort_n(int n) {
    long passes = 0;
    long total = (long)n * n;
    for (long i = 0; i < total; i++) passes++;
    return passes;
}

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    long result = bubble_sort_n(5000);
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
