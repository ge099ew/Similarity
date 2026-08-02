#include <stdio.h>
#include <time.h>

int main(void) {
    struct timespec s, e;
    clock_gettime(CLOCK_MONOTONIC, &s);
    long count = 0;
    for (int i = 0; i < 1000; i++)
        for (int j = 0; j < 1000; j++)
            count++;
    clock_gettime(CLOCK_MONOTONIC, &e);
    double ms = (e.tv_sec-s.tv_sec)*1000.0 + (e.tv_nsec-s.tv_nsec)/1e6;
    printf("result: %ld  time: %.2fms\n", count, ms);
    return 0;
}
