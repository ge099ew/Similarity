use std::time::Instant;

fn main() {
    let start = Instant::now();
    const N: i32 = 200;
    let mut sum: i32 = 0;
    for i in 0..N {
        for _j in 0..N {
            for k in 0..N {
                sum = sum.wrapping_add(i.wrapping_mul(k));
            }
        }
    }
    let ms = start.elapsed().as_secs_f64() * 1000.0;
    println!("result: {}  time: {:.2}ms", sum, ms);
}
