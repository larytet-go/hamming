Package hamming implelements multi-index minimal hammign distance algorithm
See ["Fast and compact Hamming distance index"](http://pages.di.unipi.it/rossano/wp-content/uploads/sites/7/2016/05/sigir16b.pdf) (Simon Gog, Rossano Venturini)

I have built a POC which computes ~50M/s shortest hamming distances between two 265 bits hashes. Most of the performance improvements came from three things.

* I use array of eight 64 bits words to keep hashes and can calculate a hamming distance between two 256 hashes in 20ns or 50M hashes/s on a single i7 core. 
* I keep two tables - one for search and another for updates. After an update I switch the tables. The code runs lock free.


# API

The add/remove and dup API can not be called simultaneously. APIs add/remove modify the hash tables. APIs dup/distance only read the hash tables. 
Usually the applciation will duplicate the H(amming) object and switch the pointer to the instance

```Go
    var currentH *hamming.H          // all threads use this instance
    {
        newH := currentH.Dup()       // clone the hash tables
        newH.AddBulk(allMyNewHashes)
        currentH = newH              // Let's switch global pointer to the Hamming object
    }
```

# Benchmarks

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


# Links

* https://github.com/dutchcoders/gossdeep
* https://ssdeep-project.github.io/ssdeep/
* https://github.com/glaslos/ssdeep
* https://towardsdatascience.com/fuzzy-matching-at-scale-84f2bfd0c536
* https://medium.com/@glaslos/locality-sensitive-fuzzy-hashing-66127178ebdc
* https://github.com/glaslos/tlsh




# Sample

```
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net20/Newtonsoft.Json.xml nuget/newtonsoft.json.12.0.1.nupkg/lib/net20/Newtonsoft.Json.xml 6
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net35/Newtonsoft.Json.dll nuget/newtonsoft.json.12.0.1.nupkg/lib/net35/Newtonsoft.Json.dll 62
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net45/Newtonsoft.Json.xml nuget/newtonsoft.json.12.0.1.nupkg/lib/net40/Newtonsoft.Json.xml 18
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/netstandard2.0/Newtonsoft.Json.xml nuget/newtonsoft.json.12.0.1.nupkg/lib/netstandard1.0/Newtonsoft.Json.xml 4
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/portable-net40+sl5+win8+wp8+wpa81/Newtonsoft.Json.dll nuget/newtonsoft.json.12.0.1.nupkg/lib/netstandard1.3/Newtonsoft.Json.dll 60
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/packageIcon.png nuget/newtonsoft.json.13.0.1.nupkg/packageIcon.png 0
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net20/Newtonsoft.Json.dll nuget/newtonsoft.json.13.0.1.nupkg/lib/net20/Newtonsoft.Json.dll 66
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net20/Newtonsoft.Json.xml nuget/newtonsoft.json.13.0.1.nupkg/lib/net20/Newtonsoft.Json.xml 4
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net35/Newtonsoft.Json.dll nuget/newtonsoft.json.13.0.1.nupkg/lib/net35/Newtonsoft.Json.dll 50
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net35/Newtonsoft.Json.xml nuget/newtonsoft.json.13.0.1.nupkg/lib/net35/Newtonsoft.Json.xml 0
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net40/Newtonsoft.Json.dll nuget/newtonsoft.json.13.0.1.nupkg/lib/net40/Newtonsoft.Json.dll 36
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net40/Newtonsoft.Json.xml nuget/newtonsoft.json.13.0.1.nupkg/lib/net40/Newtonsoft.Json.xml 0
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net45/Newtonsoft.Json.dll nuget/newtonsoft.json.13.0.1.nupkg/lib/net45/Newtonsoft.Json.dll 62
    hamming_test.go:91: nuget/newtonsoft.json.12.0.3.nupkg/lib/net45/Newtonsoft.Json.xml nuget/newtonsoft.json.13.0.1.nupkg/lib/net45/Newtonsoft.Json.xml 0
```