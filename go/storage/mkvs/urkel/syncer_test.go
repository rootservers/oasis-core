package urkel

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasislabs/ekiden/go/common"
	"github.com/oasislabs/ekiden/go/common/cbor"
	"github.com/oasislabs/ekiden/go/common/crypto/hash"
	"github.com/oasislabs/ekiden/go/storage/mkvs/urkel/node"
	"github.com/oasislabs/ekiden/go/storage/mkvs/urkel/syncer"
)

func TestProof(t *testing.T) {
	require := require.New(t)

	// Build a simple in-memory Merkle tree.
	ctx := context.Background()
	keys, values := generateKeyValuePairsEx("", 10)
	var ns common.Namespace

	tree := New(nil, nil)
	for i, key := range keys {
		err := tree.Insert(ctx, key, values[i])
		require.NoError(err, "Insert")
	}
	_, rootHash, err := tree.Commit(ctx, ns, 0)
	require.NoError(err, "Commit")

	// Create a Merkle proof, starting at the root node.
	builder := syncer.NewProofBuilder(rootHash)
	require.False(builder.HasRoot(), "HasRoot should return false")
	require.EqualValues(rootHash, builder.GetRoot(), "GetRoot should return correct root")

	_, err = builder.Build(ctx)
	require.Error(err, "Build should fail without a root present")

	// Including a nil node should not panic.
	builder.Include(nil)

	// Include root node.
	rootNode := tree.cache.pendingRoot.Node
	builder.Include(rootNode)
	require.True(builder.HasRoot(), "HasRoot should return true after root included")

	proof, err := builder.Build(ctx)
	require.NoError(err, "Build should not fail")
	require.EqualValues(proof.UntrustedRoot, rootHash, "UntrustedRoot should be correct")
	require.Len(proof.Entries, 3, "proof should only contain the root and two child hashes")

	// Include root.left node.
	rootIntNode := rootNode.(*node.InternalNode)
	leftNode1 := rootIntNode.Left.Node
	builder.Include(leftNode1)

	proof, err = builder.Build(ctx)
	require.NoError(err, "Build should not fail")
	// Pre-order: root(full), root.left(full), root.left.left(hash), root.left.right(hash), root.right(hash)
	require.Len(proof.Entries, 5, "proof should only contain the correct amount of nodes")
	require.EqualValues(proof.Entries[0][0], 0x01, "first entry should be a full node")
	require.EqualValues(proof.Entries[1][0], 0x01, "second entry should be a full node")
	require.EqualValues(proof.Entries[2][0], 0x02, "third entry should be a hash")
	require.EqualValues(proof.Entries[3][0], 0x02, "fourth entry should be a hash")
	require.EqualValues(proof.Entries[4][0], 0x02, "fifth entry should be a hash")

	decNode, err := node.UnmarshalBinary(proof.Entries[0][1:])
	require.NoError(err, "first entry should unmarshal as a node")
	decIntNode, ok := decNode.(*node.InternalNode)
	require.True(ok, "first entry must be an internal node (root)")
	require.Nil(decIntNode.Left, "first entry must use compact encoding")
	require.Nil(decIntNode.Right, "first entry must use compact encoding")

	decNode, err = node.UnmarshalBinary(proof.Entries[1][1:])
	require.NoError(err, "second entry should unmarshal as a node")
	decIntNode, ok = decNode.(*node.InternalNode)
	require.True(ok, "second entry must be an internal node (root.left)")
	require.Nil(decIntNode.Left, "second entry must use compact encoding")
	require.Nil(decIntNode.Right, "second entry must use compact encoding")

	leftIntNode1 := leftNode1.(*node.InternalNode)
	require.EqualValues(leftIntNode1.Left.Hash[:], proof.Entries[2][1:], "third entry hash should be correct (root.left.left)")
	require.EqualValues(leftIntNode1.Right.Hash[:], proof.Entries[3][1:], "fourth entry hash should be correct (root.left.left)")
	require.EqualValues(rootIntNode.Right.Hash[:], proof.Entries[4][1:], "fifth entry hash should be correct (root.right)")

	// Proof should be stable.
	// TODO: Provide multiple test vectors.
	testVectorProof := base64.StdEncoding.EncodeToString(cbor.Marshal(proof))
	require.EqualValues(
		"omdlbnRyaWVzhVIBAQAAAAAAAAAAJABrZXkgMAJOAQEAAAAAAAAAAAEAAAJYIQIQb3/oa32LwFDPgWs981ShL0gbPqt1ukBp6HbjH"+
			"/Wz81ghAqDH7XAay7FXPD3A1Jjerq2VJ3+qXKpDmsn2GZaRC/MyWCEC/pte6Ci+YRcj5qqf30hjTTdsnnSLQYRJJuDntH47+SdudW"+
			"50cnVzdGVkX3Jvb3RYIPGqFcpFKzYGSKFyVv70CXCpkr2XLQYsuTu0DHywQ/TJ",
		testVectorProof,
	)
	testVectorRootHash := rootHash.String()
	require.EqualValues("f1aa15ca452b360648a17256fef40970a992bd972d062cb93bb40c7cb043f4c9", testVectorRootHash)

	// Proof should verify.
	var pv syncer.ProofVerifier
	_, err = pv.VerifyProof(ctx, rootHash, proof)
	require.NoError(err, "VerifyProof should not fail with a valid proof")

	// Invalid proofs should not verify.

	// Different root.
	var bogusHash hash.Hash
	bogusHash.FromBytes([]byte("i am a bogus hash"))
	_, err = pv.VerifyProof(ctx, bogusHash, proof)
	require.Error(err, "VerifyProof should fail with proof for a different root")

	// Different hash element.
	corrupted := *proof
	corrupted.Entries[4][10] = 0x00
	_, err = pv.VerifyProof(ctx, rootHash, &corrupted)
	require.Error(err, "VerifyProof should fail with invalid proof")

	// Corrupted full node.
	corrupted = *proof
	corrupted.Entries[0] = corrupted.Entries[0][:3]
	_, err = pv.VerifyProof(ctx, rootHash, &corrupted)
	require.Error(err, "VerifyProof should fail with invalid proof")

	// Corrupted hash.
	corrupted = *proof
	corrupted.Entries[2] = corrupted.Entries[2][:3]
	_, err = pv.VerifyProof(ctx, rootHash, &corrupted)
	require.Error(err, "VerifyProof should fail with invalid proof")

	// Corrupted proof element type.
	corrupted = *proof
	corrupted.Entries[3][0] = 0xaa
	_, err = pv.VerifyProof(ctx, rootHash, &corrupted)
	require.Error(err, "VerifyProof should fail with invalid proof")
}