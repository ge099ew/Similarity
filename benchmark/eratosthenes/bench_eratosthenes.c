#include <stdio.h>
#include <time.h>

static int is_prime(int n) {
    if (n <= 1) return 0;
    for (int i = 2; i <= n; i++)
        if (i != n && n % i == 0) return 0;
    return 1;
}

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    int count = 0;
    for (int i = 2; i <= 10000; i++)
        if (is_prime(i)) count++;
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %d  time: %.2fms\n", count, ms);
    return 0;
}
