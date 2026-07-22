#include <stdio.h>
#include <time.h>

long fibonacci(int n) {
    if (n <= 1) return n;
    return fibonacci(n-1) + fibonacci(n-2);
}

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long result = fibonacci(40);
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
