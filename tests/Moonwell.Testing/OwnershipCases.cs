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
    }
}
