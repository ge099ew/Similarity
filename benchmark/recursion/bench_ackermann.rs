use std::time::Instant;

fn ackermann(m: i64, n: i64) -> i64 {
    if m == 0 { return n + 1; }
    if n == 0 { return ackermann(m-1, 1); }
    ackermann(m-1, ackermann(m, n-1))
}

fn main() {
    let start = Instant::now();
    let result = ackermann(3, 7);
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", result, ms);
}
