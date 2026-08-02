use std::time::Instant;

fn is_prime(n: i32) -> bool {
    if n <= 1 { return false; }
    let mut i = 2;
    while i <= n {
        if i != n && n % i == 0 { return false; }
        i += 1;
    }
    true
}

fn main() {
    let start = Instant::now();
    let count = (2..=10000).filter(|&n| is_prime(n)).count();
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", count, ms);
}
