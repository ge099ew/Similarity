#include <stdio.h>
#include <time.h>

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long count = 0;
    for (int i = 0; i < 1000; i++)
        for (int j = 0; j < 1000; j++)
            count++;
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("result: %ld  time: %.2fms\n", count, ms);
    return 0;
}
