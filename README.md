# fetcher

Note: 中控机需要能够 ssh 到对应的 ip 的机器上, 且该脚本无法在 Mac/Windows 上运行

## Config

```yaml
address:
  - status_port: 23211
    ip: 172.16.5.34
  - status_port: 10080
    ip: 127.0.0.1

bounds:
  - type: speed
    DeltaSecs: 30
    DeltaMB: 25
  - type: quantity
    proportion: 0.6

user: tidb
```

* speed: 在 DeltaSecs 秒内增加了 DeltaMB 的内存，会抓取火焰图
* quantity: TiDB 占用了机器 proportion% 的内存，会抓取火焰图

## Run

```
./fetcher -config=config.yaml -output=perf
```


