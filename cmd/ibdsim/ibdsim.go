/* test the utreexo forest */

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/mit-dci/utreexo/utreexo"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var maxmalloc uint64
var genproofs = flag.Bool("genproofs", false, "Generate proofs")
var genhist = flag.Bool("genhist", false, "Generate histogram")

var ttlfn = flag.String("ttlfn", "ttl.mainnet.txos", "ttl filename")

func main() {
	flag.Parse()
	if *genproofs {
		fmt.Println("Building proofs...")
		err := buildProofs()
		if err != nil {
			panic(err)
		}
	}
	if *genhist {
		err := histogram()
		if err != nil {
			panic(err)
		}
		return
	}
	err := runIBD()
	if err != nil {
		panic(err)
	}

}

// run IBD from block proof data
// we get the new utxo info from the same txos text file
// the deletion data and proofs though, we get from the leveldb
// which was created by the bridge node.
func runIBD() error {
	txofile, err := os.OpenFile(*ttlfn, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}

	defer txofile.Close()

	proofDB, err := leveldb.OpenFile("./proofdb", &opt.Options{ReadOnly: true})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(txofile)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB should be enough?

	var height uint32
	height = 1

	var plustime time.Duration
	starttime := time.Now()

	totalTXOAdded := 0
	totalDels := 0

	var blockAdds []utreexo.LeafTXO
	var blockDels []utreexo.Hash

	var p utreexo.Pollard

	p.Minleaves = 100000
	p.Lookahead = 3000

	for scanner.Scan() {
		switch scanner.Text()[0] {
		case '-':
			// blarg, still need to read these for the dedupe part
			h := utreexo.HashFromString(scanner.Text()[1:])
			blockDels = append(blockDels, h)

		case '+':
			plusstart := time.Now()

			adds, err := plusLine(scanner.Text())
			if err != nil {
				return err
			}
			blockAdds = append(blockAdds, adds...)
			donetime := time.Now()
			plustime += donetime.Sub(plusstart)

		case 'h':
			// dedupe, though in this case we don't care about dels,
			// just don't want to add stuff that shouldn't be there
			utreexo.DedupeHashSlices(&blockAdds, &blockDels)

			// read a block proof from the db
			bpBytes, err := proofDB.Get(utreexo.U32tB(height), nil)
			if err != nil {
				return err
			}

			bp, err := utreexo.FromBytesBlockProof(bpBytes)
			if err != nil {
				return err
			}

			err = p.IngestBlockProof(bp)
			if err != nil {
				return err
			}

			totalTXOAdded += len(blockAdds)
			totalDels += len(bp.Targets)

			err = p.Modify(blockAdds, bp.Targets)
			if err != nil {
				return err
			}

			if height%100 == 0 {
				fmt.Printf("Block %d add %d del %d %s plus %.2f total %.2f \n",
					height, totalTXOAdded, totalDels, p.Stats(),
					plustime.Seconds(), time.Now().Sub(starttime).Seconds())
			}
			if height%1000 == 0 {
				fmt.Printf(MemStatString())
			}

			blockAdds = []utreexo.LeafTXO{}
			blockDels = []utreexo.Hash{}
			height++
		default:
			panic("unknown string")
		}
	}

	err = proofDB.Close()
	if err != nil {
		return err
	}

	return scanner.Err()

	return nil
}

// build the bridge node / proofs
func buildProofs() error {

	fmt.Println(*ttlfn)
	txofile, err := os.OpenFile(*ttlfn, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}

	proofDB, err := leveldb.OpenFile("./proofdb", nil)
	if err != nil {
		return err
	}

	f := utreexo.NewForest()

	scanner := bufio.NewScanner(txofile)

	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB should be enough?

	var height uint32
	var totalProofNodes int
	var plustime time.Duration
	starttime := time.Now()

	var blockAdds []utreexo.LeafTXO
	var blockDels []utreexo.Hash

	for scanner.Scan() {
		switch scanner.Text()[0] {
		case '-':
			h := utreexo.HashFromString(scanner.Text()[1:])
			//			fmt.Printf("%s -> %x\n", scanner.Text(), h)
			blockDels = append(blockDels, h)

		case '+':
			plusstart := time.Now()

			lineAdds, err := plusLine(scanner.Text())
			if err != nil {
				return err
			}

			blockAdds = append(blockAdds, lineAdds...)

			donetime := time.Now()
			plustime += donetime.Sub(plusstart)

		case 'h':

			utreexo.DedupeHashSlices(&blockAdds, &blockDels)

			height++

			// get set of currently known hashes here

			blockProof, err := f.ProveBlock(blockDels)
			if err != nil {
				return fmt.Errorf("block %d %s %s", height, f.Stats(), err.Error())
			}

			ok := f.VerifyBlockProof(blockProof)
			if !ok {
				return fmt.Errorf("VerifyBlockProof failed at block %d", height)
			}

			totalProofNodes += len(blockProof.Proof)
			err = proofDB.Put(
				utreexo.U32tB(uint32(height)), blockProof.ToBytes(), nil)
			if err != nil {
				return err
			}

			//			for _, p := range proofs {
			//				ok := f.Verify(p)
			//				if !ok {
			//					return fmt.Errorf("proof position %p failed", p.Position)
			//				}
			//				fmt.Printf("proof %d: pos %d %d sibs %v\n",
			//					i, p.Position, len(p.Siblings), ok)

			//			}

			//			err := doReads(dels)
			//			if err != nil {
			//				return err
			//			}

			//			fmt.Printf("----------------------- call modify for block %d\n", height)

			err = f.Modify(blockAdds, blockProof.Targets)
			if err != nil {
				return err
			}

			// evict hashes from hashStash if they've been deleted
			//			for _, d := range dels {
			//				delete(hashStash, d)
			//			}

			blockAdds = []utreexo.LeafTXO{}
			blockDels = []utreexo.Hash{}
			//			fmt.Printf("done with block %d\n", height)

			if height%100 == 0 {
				fmt.Printf("Block %d %s plus %.2f total %.2f proofnodes %d \n",
					height, f.Stats(),
					plustime.Seconds(), time.Now().Sub(starttime).Seconds(),
					totalProofNodes)
			}

		default:
			panic("unknown string")
		}

	}
	err = proofDB.Close()
	if err != nil {
		return err
	}
	return scanner.Err()

}

func MemStatString() string {
	var s string
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if m.Alloc > maxmalloc {
		maxmalloc = m.Alloc

		// overwrite profile to get max mem usage
		// (only measured at 1000 block increments...)
		memfile, err := os.Create("memprof")
		if err != nil {
			panic(err.Error())
		}
		pprof.WriteHeapProfile(memfile)
		memfile.Close()
	}
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	s = fmt.Sprintf("alloc %d MB max %d MB", m.Alloc>>20, maxmalloc>>20)
	s += fmt.Sprintf("\ttotalAlloc %d MB", m.TotalAlloc>>20)
	s += fmt.Sprintf("\tsys %d MB", m.Sys>>20)
	s += fmt.Sprintf("\tnumGC %d\n", m.NumGC)
	return s
}
