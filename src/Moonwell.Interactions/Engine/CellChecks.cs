using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>The cell checks of the units and items tracked: one list of their entries, read round and round, a few at each step, so that each is read again once in every CellCheck seconds. The list is reused, and a step makes no table.</summary>
    internal sealed class CellChecks : IDue
    {
        /// <summary>The entries of the tracked units and items, in the order they were tracked.</summary>
        private readonly List<Entry> ring = new List<Entry>();

        /// <summary>The index in the list of the entry to read next.</summary>
        private int cursor;

        /// <summary>The step of the next check; -1 while the list is empty and nothing is on the cycle's ring.</summary>
        private int dueStep = -1;

        /// <summary>How many entries the list holds.</summary>
        public int Count
        {
            get { return ring.Count; }
        }

        /// <summary>Cell checks run in the second pass, after the gizmos and before the interactions.</summary>
        public int Pass
        {
            get { return 1; }
        }

        /// <summary>The step of the next check; -1 when there is none.</summary>
        public int DueStep
        {
            get { return dueStep; }
        }

        /// <summary>How many entries a step reads of a list of this many: enough to read them all in CellCheck seconds, rounded up, at least 1 and at most all of them. A thousandth is forgiven, since a count that comes out a whole number can come out a little over in a float.</summary>
        public static int ReadsFor(int count)
        {
            var exact = count * Interactions.Step / Interactions.CellCheck - 0.001f;
            var reads = (int)exact;
            if (reads < exact)
            {
                reads++;
            }

            if (reads < 1)
            {
                reads = 1;
            }

            return reads > count ? count : reads;
        }

        /// <summary>Puts an entry at the end of the list, and puts the checks on the cycle's ring if they are not on it.</summary>
        public void Add(Entry entry)
        {
            entry.Ring = ring.Count;
            ring.Add(entry);
            Arm();
        }

        /// <summary>Takes an entry out of the list in a few moves, with no search, as the entry keeps its index: the entries read in this round stay before the place the round has got to and those not read yet after it, so each entry left is still read once in the round. The order of the rest changes, the same way on every machine. An entry not in the list is passed over.</summary>
        public void Remove(Entry entry)
        {
            var index = entry.Ring;
            if (index < 0 || index >= ring.Count || Raw.At(ring, index) != entry)
            {
                return;
            }

            // The cursor is always below the count, so the last entry is one not yet read in this round.
            var last = ring.Count - 1;
            if (index < cursor)
            {
                // Read in this round: the last entry read takes its place, and the last entry of the list takes the last
                // read one's, where the round reads next.
                var lastRead = cursor - 1;
                Place(index, Raw.At(ring, lastRead));
                Place(lastRead, Raw.At(ring, last));
                cursor = lastRead;
            }
            else
            {
                // Not read yet: the last entry, not read yet either, takes its place.
                Place(index, Raw.At(ring, last));
            }

            ring.RemoveAt(last);
            entry.Ring = -1;
            if (cursor >= ring.Count)
            {
                cursor = 0;
            }
        }

        /// <summary>Puts an entry at an index of the list, which it then keeps.</summary>
        private void Place(int index, Entry entry)
        {
            Raw.Put(ring, index, entry);
            entry.Ring = index;
        }

        /// <summary>Reads the next entries of the round, and puts the checks on the ring for the next step unless the list is empty. An error a read raises, the world's, ends the step's reads and is reported as Interactions.CellCheck, and the round goes on from the entry after at the next step: nothing leaves the cycle's step.</summary>
        public void RunDue(int step)
        {
            dueStep = -1;
            if (ring.Count == 0)
            {
                return;
            }

            var failure = Guard.Read(this);
            Arm();
            if (failure != null)
            {
                Errors.Report("Interactions.CellCheck", failure);
            }
        }

        /// <summary>Reads the step's share of the round. The cursor moves past an entry before it is read, so a read that fails does not stop the round at that entry.</summary>
        internal void ReadRound()
        {
            var count = ring.Count;
            var reads = ReadsFor(count);
            for (var i = 0; i < reads; i++)
            {
                var entry = Raw.At(ring, cursor);
                cursor++;
                if (cursor >= count)
                {
                    cursor = 0;
                }

                Tracking.ReadNow(entry);
            }
        }

        /// <summary>Puts the checks on the ring for the step after this one, unless they are on it or the list is empty.</summary>
        private void Arm()
        {
            if (dueStep >= 0 || ring.Count == 0)
            {
                return;
            }

            var beat = Tracking.cycle ?? Tracking.Beat;
            dueStep = beat.StepNow + 1;
            beat.Schedule(this, dueStep);
        }
    }
}
