package ankabuffer

import (
	"fmt"
	"strconv"

	"github.com/dofusdude/ankabuffer/AnkamaGames"
)

type Chunk struct {
	Hash   string `json:"hash"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	Done   bool   `json:"done"`
}

type File struct {
	Name           string   `json:"name"`
	Size           int64    `json:"size"`
	Hash           string   `json:"hash"`
	Chunks         []Chunk  `json:"chunks"`
	Executable     bool     `json:"executable"`
	Symlink        string   `json:",omitempty"`
	ReverseBundles []string `json:"reverse_bundles"` // bundles containing chunks or entire file that are needed to reconstruct this file
}

type Bundle struct {
	Hash   string  `json:"hash"`
	Chunks []Chunk `json:"chunks"`
}

type Fragment struct {
	Name    string          `json:"name"`
	Files   map[string]File `json:"files"`
	Bundles []Bundle        `json:"bundles"`
}

type Manifest struct {
	Fragments map[string]Fragment `json:"fragments"`
}

type iHashFile interface {
	Hash(j int) byte
	HashLength() int
}

func getHash[T iHashFile](file T) string {
	hash := ""
	for i := 0; i < file.HashLength(); i++ {
		hash += fmt.Sprintf("%02s", strconv.FormatInt(int64(file.Hash(i)), 16))
	}
	return hash
}

func ParseManifest(data []byte) *Manifest {
	flatbManifest := AnkamaGames.GetRootAsManifest(data, 0)
	manifest := Manifest{}
	manifest.Fragments = make(map[string]Fragment)

	bundleLookup := make(map[string]Bundle)
	for i := 0; i < flatbManifest.FragmentsLength(); i++ {
		fragment := AnkamaGames.Fragment{}
		flatbManifest.Fragments(&fragment, i)

		for j := 0; j < fragment.BundlesLength(); j++ {
			bundle := AnkamaGames.Bundle{}
			fragment.Bundles(&bundle, j)
			bundleJson := Bundle{}
			bundleJson.Hash = getHash(&bundle)
			bundleJson.Chunks = make([]Chunk, bundle.ChunksLength())
			for k := 0; k < bundle.ChunksLength(); k++ {
				chunk := AnkamaGames.Chunk{}
				bundle.Chunks(&chunk, k)
				chunkJson := Chunk{}
				chunkJson.Hash = getHash(&chunk)
				chunkJson.Offset = chunk.Offset()
				chunkJson.Size = chunk.Size()
				bundleJson.Chunks[k] = chunkJson
			}
			bundleLookup[bundleJson.Hash] = bundleJson
		}
	}

	for i := 0; i < flatbManifest.FragmentsLength(); i++ {
		fragment := AnkamaGames.Fragment{}
		flatbManifest.Fragments(&fragment, i)

		fragmentJson := Fragment{}
		fragmentJson.Files = make(map[string]File)
		fragmentJson.Name = string(fragment.Name())
		fragmentJson.Bundles = make([]Bundle, fragment.BundlesLength())
		for j := 0; j < fragment.BundlesLength(); j++ {
			bundle := AnkamaGames.Bundle{}
			fragment.Bundles(&bundle, j)
			fragmentJson.Bundles[j] = bundleLookup[getHash(&bundle)]
		}

		for j := 0; j < fragment.FilesLength(); j++ {
			file := AnkamaGames.File{}
			fragment.Files(&file, j)
			fileJson := File{}
			fileJson.Name = string(file.Name())
			fileJson.Size = file.Size()
			fileJson.Hash = getHash(&file)
			fileJson.Chunks = make([]Chunk, file.ChunksLength())
			for k := 0; k < file.ChunksLength(); k++ {
				chunk := AnkamaGames.Chunk{}
				file.Chunks(&chunk, k)
				chunkJson := Chunk{}
				chunkJson.Hash = getHash(&chunk)
				chunkJson.Offset = chunk.Offset()
				chunkJson.Size = chunk.Size()
				fileJson.Chunks[k] = chunkJson
			}
			fileJson.Executable = file.Executable() == 1
			if file.Symlink() != nil {
				fileJson.Symlink = string(file.Symlink())
			}
			fragmentJson.Files[fileJson.Name] = fileJson
		}
		manifest.Fragments[fragmentJson.Name] = fragmentJson
	}

	// add reverse bundles searching in all fragments
	for _, fragment := range manifest.Fragments {
		for fileIdx, file := range fragment.Files {
			realFile := file
			bundles := NewSet[string]()
			for _, searchFragment := range manifest.Fragments {
				for _, bundle := range searchFragment.Bundles {
					for _, chunk := range bundle.Chunks {
						if len(file.Chunks) == 0 {
							if chunk.Hash == file.Hash {
								realFile.ReverseBundles = []string{bundle.Hash}
								break
							}
						} else {
							for _, fileChunk := range file.Chunks {
								if chunk.Hash == fileChunk.Hash {
									bundles.Add(bundle.Hash)
								}
							}
						}
					}
				}
				if bundles.Size() == 0 {
					realFile.ReverseBundles = nil
				} else {
					realFile.ReverseBundles = bundles.Slice()
				}
				fragment.Files[fileIdx] = realFile
			}
		}
	}
	return &manifest
}

func GetNeededBundles(files []File) []string {
	bundles := NewSet[string]()
	for _, file := range files {
		if file.ReverseBundles != nil {
			bundles.AddMulti(file.ReverseBundles...)
		}
	}
	return bundles.Slice()
}

func GetBundleHashMap(manifest *Manifest) map[string]Bundle {
	bundleHashMap := make(map[string]Bundle)
	for _, fragment := range manifest.Fragments {
		for _, bundle := range fragment.Bundles {
			bundleHashMap[bundle.Hash] = bundle
		}
	}
	return bundleHashMap
}
