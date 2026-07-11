#include <stdio.h>
#include <stdlib.h>
#include <time.h>

// Similarity - Application
// value: Test
// type: AllFeatures

long test_pointer() {
    int x = 42;
    return 0;
}

long test_cast() {
    x = 10;
    float y = 0;
    return 0;
}

long test_risk() {
    x = 99;
    return 0;
}

long test_overflow_check() {
    x = 2147483647;
    return 0;
}

long sim_main() {
    int a = 0;
    int b = 0;
    int c = 0;
    int d = 0;
    return 0;
}

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long result = sim_main();
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("Similarity result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
