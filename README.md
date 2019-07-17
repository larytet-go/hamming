Package hamming implelements multi-index minimal hammign distance algorithm
See "Fast and compact Hamming distance index" (Simon Gog, Rossano Venturini) http://pages.di.unipi.it/rossano/wp-content/uploads/sites/7/2016/05/sigir16b.pdf

The add/remove/dup API  is not reentrant.
APIs add/remove modify the hash tables
APIs dup/distance only read the hash tables

Usually the applciation will duplicate the H(amming) object and switch the pointer to the instance

```Go
    var currentH *hamming.H          // all threads use this instance
    {
        newH := currentH.Dup()       // clone the hash tables
        newH.AddBulk(allMyNewHashes)
        currentH = newH              // Let's switch global pointer to the Hamming object
    }
```


Benchmarks for 256 bits hashes 
```
BenchmarkRealDataSet-4             	 5000000	       341 ns/op
BenchmarkRealDataSet100K-4         	      30	  34574061 ns/op
BenchmarkRealDataSet1M-4           	       3	 343505768 ns/op
BenchmarkHammingAdd-4              	  500000	      2646 ns/op
BenchmarkFuzzyHashToKey-4          	2000000000	         0.30 ns/op
BenchmarkFuzzyHashToString-4       	 2000000	       805 ns/op
BenchmarkClosestSibling-4          	10000000	       137 ns/op
BenchmarkClosestSibling2K-4        	   50000	     31350 ns/op
BenchmarkClosestSibling100K-4      	    1000	   1904851 ns/op
BenchmarkClosestSibling1M-4        	     100	  16311071 ns/op
BenchmarkHammingDistance-4         	50000000	        25.1 ns/op
BenchmarkHashStringToFuzzyHash-4   	10000000	       174 ns/op
```