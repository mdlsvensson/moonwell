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
            ring.Add(entry);
            Arm();
        }

        /// <summary>Takes an entry out of the list, keeping the order of the rest and the place the round has got to.</summary>
        public void Remove(Entry entry)
        {
            var index = ring.IndexOf(entry);
            if (index < 0)
            {
                return;
            }

            ring.RemoveAt(index);
            if (index < cursor)
            {
                cursor--;
            }

            if (cursor >= ring.Count)
            {
                cursor = 0;
            }
        }

        /// <summary>Reads the next entries of the round, and puts the checks on the ring for the next step unless the list is empty.</summary>
        public void RunDue(int step)
        {
            dueStep = -1;
            var count = ring.Count;
            if (count == 0)
            {
                return;
            }

            var reads = ReadsFor(count);
            for (var i = 0; i < reads; i++)
            {
                Tracking.ReadNow(ring[cursor]);
                cursor++;
                if (cursor >= count)
                {
                    cursor = 0;
                }
            }

            Arm();
        }

        /// <summary>Puts the checks on the ring for the step after this one, unless they are on it or the list is empty.</summary>
        private void Arm()
        {
            if (dueStep >= 0 || ring.Count == 0)
            {
                return;
            }

            var beat = Tracking.Beat;
            dueStep = beat.StepNow + 1;
            beat.Schedule(this, dueStep);
        }
    }
}
