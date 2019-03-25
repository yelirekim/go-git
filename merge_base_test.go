package git

import (
	"sort"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git-fixtures.v3"
)

func commitsFromRevs(repo *Repository, revs []string) ([]*object.Commit, error) {
	var commits []*object.Commit
	for _, rev := range revs {
		hash, err := repo.ResolveRevision(plumbing.Revision(rev))
		if err != nil {
			return nil, err
		}

		commit, err := repo.CommitObject(*hash)
		if err != nil {
			return nil, err
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

func alphabeticSortCommits(commits []*object.Commit) {
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Hash.String() > commits[j].Hash.String()
	})
}

/*

The following tests consider this history having two root commits: V and W

V---o---M----AB----A---CD1--R---C--------S-------------------Q < master
               \         \ /            /                   /
                \         X            GQ1---G < feature   /
                 \       / \          /     /             /
W---o---N----o----B---CD2---o---D----o----GQ2------------o < dev

MergeBase
----------------------------
passed  merge-base
 M, N               Commits with unrelated history, have no merge-base
 A, B    AB         Regular merge-base between two commits
 A, A    A          The merge-commit between equal commits, is the same
 Q, N    N          The merge-commit between a commit an its ancestor, is the ancestor
 C, D    CD1, CD2   Cross merges causes more than one merge-base
 G, Q    GQ1, GQ2   Feature branches including merges, causes more than one merge-base

Independents
----------------------------
candidates        result
 A                 A           Only one commit returns it
 A, A, A           A           Repeated commits are ignored
 A, A, M, M, N     A, N        M is reachable from A, no it is not independent
 CD1, CD2, M, N    CD1, CD2    M and N are reachable from CD2, so they're not
 C, G, dev, M, N   C, G, dev   M and N are reachable from G, so they're not
 A, A^, A, N, N^   A, N        A^ and N^ are rechable from A and N


IsAncestor
----------------------------
passed   result
 A^^, A   true      Will be true if first is ancestor of the second
 M, G     true      True because it will also reach G from M crossing merge commits
 A, A     true      True if first and second are the same
 M, N     false     Commits with unrelated history, will return false
*/

var _ = Suite(&orphansSuite{})

type orphansSuite struct {
	BaseSuite
}

func (s *orphansSuite) SetUpSuite(c *C) {
	s.Suite.SetUpSuite(c)
	f := fixtures.ByTag("merge-base").One()
	s.Repository = s.NewRepository(f)
}

// AssertMergeBase validates that the merge-base of the passed revs,
// matches the expected result
func (s *orphansSuite) AssertMergeBase(c *C, revs, expectedRevs []string) {
	c.Assert(revs, HasLen, 2)

	commits, err := commitsFromRevs(s.Repository, revs)
	c.Assert(err, IsNil)

	results, err := MergeBase(commits[0], commits[1])
	c.Assert(err, IsNil)

	expected, err := commitsFromRevs(s.Repository, expectedRevs)
	c.Assert(err, IsNil)

	c.Assert(results, HasLen, len(expected))

	alphabeticSortCommits(results)
	alphabeticSortCommits(expected)
	for i, commit := range results {
		c.Assert(commit.Hash.String(), Equals, expected[i].Hash.String())
	}
}

// AssertIndependents validates the independent commits of the passed list
func (s *orphansSuite) AssertIndependents(c *C, revs, expectedRevs []string) {
	commits, err := commitsFromRevs(s.Repository, revs)
	c.Assert(err, IsNil)

	results, err := Independents(commits)
	c.Assert(err, IsNil)

	expected, err := commitsFromRevs(s.Repository, expectedRevs)
	c.Assert(err, IsNil)

	c.Assert(results, HasLen, len(expected))

	alphabeticSortCommits(results)
	alphabeticSortCommits(expected)
	for i, commit := range results {
		c.Assert(commit.Hash.String(), Equals, expected[i].Hash.String())
	}
}

// AssertAncestor validates the independent commits of the passed list
func (s *orphansSuite) AssertAncestor(c *C, revs []string, shouldBeAncestor bool) {
	c.Assert(revs, HasLen, 2)

	commits, err := commitsFromRevs(s.Repository, revs)
	c.Assert(err, IsNil)

	isAncestor, err := IsAncestor(commits[0], commits[1])
	c.Assert(err, IsNil)
	c.Assert(isAncestor, Equals, shouldBeAncestor)
}

// TestNoAncestorsWhenNoCommonHistory validates that merge-base returns no commits
// when there is no common history (M, N -> none)
func (s *orphansSuite) TestNoAncestorsWhenNoCommonHistory(c *C) {
	revs := []string{"M", "N"}
	nothing := []string{}
	s.AssertMergeBase(c, revs, nothing)
}

// TestCommonAncestorInMergedOrphans validates that merge-base returns a common
// ancestor in orphan branches when they where merged (A, B -> AB)
func (s *orphansSuite) TestCommonAncestorInMergedOrphans(c *C) {
	revs := []string{"A", "B"}
	expectedRevs := []string{"AB"}
	s.AssertMergeBase(c, revs, expectedRevs)
}

// TestMergeBaseWithSelf validates that merge-base between equal commits, returns
// the same commit (A, A -> A)
func (s *orphansSuite) TestMergeBaseWithSelf(c *C) {
	revs := []string{"A", "A"}
	expectedRevs := []string{"A"}
	s.AssertMergeBase(c, revs, expectedRevs)
}

// TestMergeBaseWithAncestor validates that merge-base between a commit an its
// ancestor returns the ancestor (Q, N -> N)
func (s *orphansSuite) TestMergeBaseWithAncestor(c *C) {
	revs := []string{"Q", "N"}
	expectedRevs := []string{"N"}
	s.AssertMergeBase(c, revs, expectedRevs)
}

// TestDoubleCommonAncestorInCrossMerge validates that merge-base returns two
// common ancestors when there are cross merges (C, D -> CD1, CD2)
func (s *orphansSuite) TestDoubleCommonAncestorInCrossMerge(c *C) {
	revs := []string{"C", "D"}
	expectedRevs := []string{"CD1", "CD2"}
	s.AssertMergeBase(c, revs, expectedRevs)
}

// TestDoubleCommonInSubFeatureBranches validates that merge-base returns two
// common ancestors when two branches where partially merged (G, Q -> GQ1, GQ2)
func (s *orphansSuite) TestDoubleCommonInSubFeatureBranches(c *C) {
	revs := []string{"G", "Q"}
	expectedRevs := []string{"GQ1", "GQ2"}
	s.AssertMergeBase(c, revs, expectedRevs)
}

// TestIndependentOnlyOne validates that Independents for one commit returns
// that same commit (A -> A)
func (s *orphansSuite) TestIndependentOnlyOne(c *C) {
	revs := []string{"A"}
	expectedRevs := []string{"A"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentOnlyRepeated validates that Independents for one repeated commit
// returns that same commit (A, A, A -> A)
func (s *orphansSuite) TestIndependentOnlyRepeated(c *C) {
	revs := []string{"A", "A", "A"}
	expectedRevs := []string{"A"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentWithRepeatedAncestors validates that Independents works well
// when there are repeated ancestors (A, A, M, M, N -> A, N)
func (s *orphansSuite) TestIndependentWithRepeatedAncestors(c *C) {
	revs := []string{"A", "A", "M", "M", "N"}
	expectedRevs := []string{"A", "N"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentBeyondShortcut validates that Independents does not stop walking
// in all paths when one of them is known (S, G, P -> S, G)
func (s *orphansSuite) TestIndependentBeyondShortcut(c *C) {
	revs := []string{"S", "G", "P"}
	expectedRevs := []string{"S", "G"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentBeyondShortcutBis validates that Independents does not stop walking
// in all paths when one of them is known (CD1, CD2, M, N -> CD1, CD2)
func (s *orphansSuite) TestIndependentBeyondShortcutBis(c *C) {
	revs := []string{"CD1", "CD2", "M", "N"}
	expectedRevs := []string{"CD1", "CD2"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentWithPairOfAncestors validates that Independents excluded all
// the ancestors (C, D, M, N -> C, D)
func (s *orphansSuite) TestIndependentWithPairOfAncestors(c *C) {
	revs := []string{"C", "D", "M", "N"}
	expectedRevs := []string{"C", "D"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentAcrossCrossMerges validates that Independents works well
// along cross merges (C, G, dev, M -> C, G, dev)
func (s *orphansSuite) TestIndependentAcrossCrossMerges(c *C) {
	revs := []string{"C", "G", "dev", "M", "N"}
	expectedRevs := []string{"C", "G", "dev"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestIndependentChangingOrder validates that Independents works well
// when the order and repetition is tricky (A, A^, A, N, N^ -> A, N)
func (s *orphansSuite) TestIndependentChangingOrder(c *C) {
	revs := []string{"A", "A^", "A", "N", "N^"}
	expectedRevs := []string{"A", "N"}
	s.AssertIndependents(c, revs, expectedRevs)
}

// TestAncestor validates that IsAncestor returns true if walking from first
// commit, through its parents, it can be reached the second ( A^^, A -> true )
func (s *orphansSuite) TestAncestor(c *C) {
	revs := []string{"A^^", "A"}
	s.AssertAncestor(c, revs, true)

	revs = []string{"A", "A^^"}
	s.AssertAncestor(c, revs, false)
}

// TestAncestorBeyondMerges validates that IsAncestor returns true also if first can be
// be reached from first one even crossing merge commits in between ( M, G -> true )
func (s *orphansSuite) TestAncestorBeyondMerges(c *C) {
	revs := []string{"M", "G"}
	s.AssertAncestor(c, revs, true)

	revs = []string{"G", "M"}
	s.AssertAncestor(c, revs, false)
}

// TestAncestorSame validates that IsAncestor returns both are the same ( A, A -> true )
func (s *orphansSuite) TestAncestorSame(c *C) {
	revs := []string{"A", "A"}
	s.AssertAncestor(c, revs, true)
}

// TestAncestorUnrelated validates that IsAncestor returns false when the passed commits
// does not share any history, no matter the order used ( M, N -> false )
func (s *orphansSuite) TestAncestorUnrelated(c *C) {
	revs := []string{"M", "N"}
	s.AssertAncestor(c, revs, false)

	revs = []string{"N", "M"}
	s.AssertAncestor(c, revs, false)
}
