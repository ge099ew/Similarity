Explanation[App{Benchmark(type:fibonacci)}]

Func[fibonacci{
  receive{int(n)},
  If[check{lesseq(n:1)},
    True[return(n)],
    False[
      Var[let{int(a:call{fibonacci(-{int(n:1)})})}],
      Var[let{int(b:call{fibonacci(-{int(n:2)})})}],
      return(+{int(a:b)})
    ]
  ]
}]

Func_pub[main{
  receive{},
  Var[let{int(result:call{fibonacci(40)})}],
  return(result)
}]
