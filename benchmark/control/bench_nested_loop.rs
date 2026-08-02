use std::time::Instant;

fn main() {
    let start = Instant::now();
    let mut count = 0i64;
    for _ in 0..1000 { for _ in 0..1000 { count += 1; } }
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", count, ms);
}
