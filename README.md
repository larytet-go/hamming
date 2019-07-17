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
