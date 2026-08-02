#include <stdio.h>
#include <time.h>

static long fibonacci(int n) {
    if (n <= 1) return n;
    return fibonacci(n-1) + fibonacci(n-2);
}

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    long result = fibonacci(40);
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
