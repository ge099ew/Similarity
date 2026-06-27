#include <iostream>
#include <chrono>

int main() {
    auto start = std::chrono::high_resolution_clock::now();

    int sum = 0;
    for (int i = 0; i < 100000000; i++) {
        sum += i;
    }

    auto end = std::chrono::high_resolution_clock::now();
    auto ms = std::chrono::duration_cast<std::chrono::microseconds>(end-start).count() / 1000.0;
    std::cout << "C++ result: " << sum << "  time: " << ms << "ms" << std::endl;
    return 0;
}
