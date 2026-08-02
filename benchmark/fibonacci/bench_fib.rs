use std::time::Instant;

fn fibonacci(n: i64) -> i64 {
    if n <= 1 { return n; }
    fibonacci(n-1) + fibonacci(n-2)
}

fn main() {
    let start = Instant::now();
    let result = fibonacci(40);
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", result, ms);
}
