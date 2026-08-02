#include <stdio.h>
#include <time.h>

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    const int N = 200;
    int sum = 0;
    for (int i = 0; i < N; i++)
        for (int j = 0; j < N; j++)
            for (int k = 0; k < N; k++)
                sum += i * k;
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %d  time: %.2fms\n", sum, ms);
    return 0;
}
