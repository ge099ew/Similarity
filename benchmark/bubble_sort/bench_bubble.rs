use std::time::Instant;

fn bubble_sort_n(n: i64) -> i64 {
    let total = n * n;
    let mut passes = 0i64;
    let mut i = 0i64;
    while i < total { passes += 1; i += 1; }
    passes
}

fn main() {
    let start = Instant::now();
    let result = bubble_sort_n(5000);
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", result, ms);
}
