#include <stdio.h>
#include <time.h>

extern int sim_main();

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long result = sim_main();
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0
              + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("Similarity result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
