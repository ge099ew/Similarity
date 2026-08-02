use std::time::Instant;

fn main() {
    let start = Instant::now();
    let mut sum: i32 = 0;
    let mut i: i32 = 0;
    while i <= 100_000_000 { sum = sum.wrapping_add(i); i += 1; }
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", sum, ms);
}
