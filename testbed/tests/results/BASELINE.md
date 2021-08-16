# Test Results
Started: Mon, 16 Dec 2019 16:09:54 -0500

Test                                    |Result|Duration|CPU Avg%|CPU Max%|RAM Avg MiB|RAM Max MiB|Sent Items|Received Items|
----------------------------------------|------|-------:|-------:|-------:|----------:|----------:|---------:|-------------:|
Metric10kDPS/SignalFx                   |PASS  |     15s|    42.7|    46.4|         22|         27|    150000|        150000|
Metric10kDPS/OpenCensus                 |PASS  |     18s|    12.0|    15.9|         28|         34|    149900|        149900|
Trace10kSPS/OpenCensus                  |PASS  |     16s|    22.5|    26.7|         35|         43|    149900|        149900|
Trace10kSPS/SAPM                        |PASS  |     16s|    27.8|    32.8|         55|         71|    149120|        149120|

Total duration: 66s