#include <stdio.h>
#include <time.h>

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long sum = 0;
    for (int i = 0; i <= 100000000; i++) {
        sum += i;
    }
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("result: %ld  time: %.2fms\n", sum, ms);
    return 0;
}
