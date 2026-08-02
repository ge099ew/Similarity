#include <stdio.h>
#include <time.h>

static long ackermann(int m, int n) {
    if (m == 0) return n + 1;
    if (n == 0) return ackermann(m-1, 1);
    return ackermann(m-1, ackermann(m, n-1));
}

int main() {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    long result = ackermann(3, 7);
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
