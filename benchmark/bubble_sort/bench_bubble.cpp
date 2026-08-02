#include <stdio.h>
#include <time.h>

// Similarity版と同じ: n*n回ループで比較回数をカウント
long bubble_sort_n(int n) {
    long passes = 0;
    long total = (long)n * n;
    for (long i = 0; i < total; i++) passes++;
    return passes;
}

int main() {
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    long result = bubble_sort_n(5000);
    clock_gettime(CLOCK_MONOTONIC, &end);
    double ms = (end.tv_sec - start.tv_sec) * 1000.0 + (end.tv_nsec - start.tv_nsec) / 1e6;
    printf("result: %ld  time: %.2fms\n", result, ms);
    return 0;
}
