use std::time::Instant;

fn inc(x: i32) -> i32 { x + 1 }

fn main() {
    let start = Instant::now();
    let mut v = 0i32;
    for _ in 0..1_000_000 { v = inc(v); }
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", v, ms);
}
