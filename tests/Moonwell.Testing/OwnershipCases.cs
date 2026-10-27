using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check owners, the order their children end in, and the current owner.</summary>
    public static class OwnershipCases
    {
        /// <summary>An owned object that adopts a new child into its owner while it ends.</summary>
        private sealed class AdoptingOwned : Owned
        {
            private readonly string name;
            private readonly List<string> log;

            /// <summary>Makes an object that logs its end and then adopts a late child into its owner.</summary>
            public AdoptingOwned(string name, List<string> log)
            {
                this.name = name;
                this.log = log;
            }

            /// <summary>Logs its end, then adopts a child named late into its owner.</summary>
            protected override void OnEnd()
            {
                log.Add("end " + name);
                Owner!.Adopt(new TestOwned("late", log));
            }
        }

        /// <summary>An owner that adopts a child from its Finish, which runs after its children have ended.</summary>
        private sealed class FinishAdopter : Owner
        {
            private readonly Owned child;

            /// <summary>Makes an owner that adopts the child when it finishes.</summary>
            public FinishAdopter(Owned child)
            {
                this.child = child;
            }

            /// <summary>Adopts the child.</summary>
            internal override void Finish()
            {
                Adopt(child);
            }
        }

        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(List<string> log, string[] expected, string what)
        {
            Expect.Equal(expected.Length, log.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], log[i], what + ": line " + i);
            }
        }

        /// <summary>An owner ends first, then its children newest first.</summary>
        public static void EndsNewestFirst(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            owner.Adopt(new TestOwned("a", log));
            owner.Adopt(new TestOwned("b", log));
            owner.Adopt(new TestOwned("c", log));
            owner.End();
            ExpectLog(log, new string[] { "end owner", "end c", "end b", "end a" }, "log after the owner ends");
        }

        /// <summary>Grandchildren end too, each before its own owner's children are done, and the owner is left empty.</summary>
        public static void EndsGrandchildren(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var middle = new TestOwner("middle", log);
            owner.Adopt(middle);
            middle.Adopt(new TestOwned("leaf", log));
            owner.End();
            ExpectLog(log, new string[] { "end owner", "end middle", "end leaf" }, "log after the owner ends");
            Expect.Equal(0, owner.OwnedCount, "owned count of the owner after it ends");
        }

        /// <summary>Ending an object twice ends it once.</summary>
        public static void EndOnce(FakeWorld world)
        {
            var log = new List<string>();
            var a = new TestOwned("a", log);
            a.End();
            a.End();
            ExpectLog(log, new string[] { "end a" }, "log after ending a twice");
        }

        /// <summary>
        /// An End started inside an OnEnd does nothing: a component whose OnRemoved removes its unit runs that unit's OnLeave and
        /// its own OnRemoved once, whether the unit is removed or the component alone.
        /// </summary>
        public static void EndInsideEnd(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h0le"), () => new LeaveCounter());
            var removed = (LeaveCounter)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h0le")))!;
            var withUnit = removed.Add(new RemovesItsUnit());
            removed.Remove();
            Expect.Equal(1, removed.Leaves, "OnLeave runs of the unit removed");
            Expect.Equal(1, withUnit.Removals, "OnRemoved runs of its component");
            Expect.True(removed.HasEnded, "the unit removed has ended");
            var kept = (LeaveCounter)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h0le")))!;
            var alone = kept.Add(new RemovesItsUnit());
            kept.Remove(alone);
            Expect.Equal(1, kept.Leaves, "OnLeave runs of the unit whose component was removed");
            Expect.Equal(1, alone.Removals, "OnRemoved runs of the component removed");
            Expect.True(kept.HasEnded && alone.HasEnded, "both have ended");
        }

        /// <summary>A unit that counts its OnLeave runs.</summary>
        private sealed class LeaveCounter : Unit
        {
            /// <summary>How many times OnLeave ran.</summary>
            public int Leaves;

            /// <summary>Counts the run.</summary>
            protected override void OnLeave()
            {
                Leaves++;
            }
        }

        /// <summary>A unit component that removes its unit when it is removed, and counts its OnRemoved runs.</summary>
        private sealed class RemovesItsUnit : UnitComponent
        {
            /// <summary>How many times OnRemoved ran.</summary>
            public int Removals;

            /// <summary>Counts the run and removes the unit.</summary>
            protected override void OnRemoved()
            {
                Removals++;
                Unit.Remove();
            }
        }

        /// <summary>An owner that has ended ends a child it adopts at once.</summary>
        public static void AdoptIntoEnded(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            owner.End();
            var child = new TestOwned("child", log);
            owner.Adopt(child);
            ExpectLog(log, new string[] { "end owner", "end child" }, "log after adopting into an ended owner");
            Expect.True(child.HasEnded, "the child has ended");
            Expect.Same(owner, child.Owner, "the child's owner after it ends");
            Expect.Equal(0, owner.OwnedCount, "owned count of the ended owner");
        }

        /// <summary>A child that ends by itself leaves its owner, and the owner's later end does not end it again.</summary>
        public static void ChildEndsAlone(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var a = new TestOwned("a", log);
            var b = new TestOwned("b", log);
            owner.Adopt(a);
            owner.Adopt(b);
            Expect.Equal(2, owner.OwnedCount, "owned count before a ends");
            a.End();
            Expect.Equal(1, owner.OwnedCount, "owned count after a ends");
            owner.End();
            ExpectLog(log, new string[] { "end a", "end owner", "end b" }, "log after a ends alone and then the owner ends");
        }

        /// <summary>A child that adopts a new child into its owner while it ends gets that child ended too.</summary>
        public static void AdoptWhileEnding(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            owner.Adopt(new TestOwned("a", log));
            owner.Adopt(new AdoptingOwned("c", log));
            owner.End();
            ExpectLog(log, new string[] { "end owner", "end c", "end late", "end a" }, "log after a child adopts while ending");
        }

        /// <summary>An error in one child's end is logged, and the other children still end.</summary>
        public static void ErrorDoesNotStop(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            owner.Adopt(new TestOwned("a", log));
            owner.Adopt(new ThrowingOwned());
            owner.Adopt(new TestOwned("b", log));
            owner.End();
            ExpectLog(log, new string[] { "end owner", "end b", "end a" }, "log after a child's end throws");
            Expect.Equal(1, world.Logged.Count, "logged lines after the throwing end");
            Expect.Equal("Moonwell: ThrowingOwned.OnEnd failed: boom", world.Logged[0], "the logged line");
        }

        /// <summary>Run makes its owner the current owner while it runs, and restores the root after, even when it throws.</summary>
        public static void RunSetsCurrent(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            Owner? seen = null;
            owner.Run(() => { seen = Owner.Current; });
            Expect.Same(owner, seen, "Owner.Current inside Run");
            Expect.Same(Runtime.Root, Owner.Current, "Owner.Current after Run");
            Expect.Throws<InvalidOperationException>(() => owner.Run(() => { throw new InvalidOperationException("boom"); }), "Run whose action throws");
            Expect.Same(Runtime.Root, Owner.Current, "Owner.Current after Run threw");
        }

        /// <summary>A child adopted from an owner's Finish, after its children have ended, ends at once.</summary>
        public static void AdoptInFinish(FakeWorld world)
        {
            var log = new List<string>();
            var child = new TestOwned("child", log);
            var owner = new FinishAdopter(child);
            owner.End();
            ExpectLog(log, new string[] { "end child" }, "log after adopting from Finish");
            Expect.True(child.HasEnded, "the child has ended");
            Expect.Equal(0, owner.OwnedCount, "owned count of the owner after it ends");
        }
    }
}
