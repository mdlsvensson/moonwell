using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>The package's one cycle: a ring of lists, three for each of the next 256 steps (one a pass), of what is due. The step makes no table: the lists are reused, a due is taken off its list by being set to a marker, and the markers are packed away at the end of the step.</summary>
    internal sealed class Cycle
    {
        /// <summary>How many steps the ring holds, a power of two, so a step's slot is the step's low bits.</summary>
        private const int Slots = 256;

        /// <summary>How many passes a step has: gizmos, cell checks, interactions.</summary>
        private const int Passes = 3;

        /// <summary>The ring: for each step modulo 256, a list for each pass, at slot * 3 + pass, of what is due, in the order it was put on. A due that runs puts itself on again, at the end of its next step's list, so the order is that of the last runs, not the order things were made. A list holds no null, so the cycle reads it raw.</summary>
        private readonly List<List<IDue>> ring = new List<List<IDue>>();

        /// <summary>What a due taken off a list leaves in its place until the list is packed: it is never due.</summary>
        private readonly IDue taken = new Taken();

        /// <summary>The step the cycle is at: the one running, or the last one that ran. It counts up from 0 over the game, and does not reset when the cycle's timer stops.</summary>
        public int StepNow;

        /// <summary>Whether a step is running now. Outside one, StepNow is the last step that ran, and a place read between two steps is read again by the next ask.</summary>
        public bool Running;

        /// <summary>Makes a cycle at step 0 with an empty list for every pass of every slot.</summary>
        public Cycle()
        {
            for (var i = 0; i < Slots * Passes; i++)
            {
                ring.Add(new List<IDue>());
            }
        }

        /// <summary>The number of steps a time in seconds takes, rounded up to whole steps and at least 1. A thousandth of a step is forgiven, since a time that is a whole number of steps can come out a little over in a float.</summary>
        public static int Steps(float seconds)
        {
            var exact = seconds / Interactions.Step - 0.001f;
            var whole = (int)exact;
            if (whole < exact)
            {
                whole++;
            }

            return whole < 1 ? 1 : whole;
        }

        /// <summary>Puts a due on its pass's list of a step, which must be after the one running, so that what a step schedules never runs in that step. A due further than 255 steps ahead waits in its slot and is passed over until its step.</summary>
        public void Schedule(IDue due, int step)
        {
            Dev.Check(step > StepNow, "Cycle.Schedule: a due must be scheduled for a step after the one running");
            var list = Raw.At(ring, (step & (Slots - 1)) * Passes + due.Pass);
            Raw.Append(list, due);
        }

        /// <summary>Runs the next step: what is due at it, in three passes, each in the order it was put on its list (the order of the last runs, not the order things were made). Each pass runs what was on its list when the step began; what a step schedules is for a later step. A due that is not due until a later turn of the ring is passed over, and one with nothing more to run is dropped.</summary>
        public void Run()
        {
#if DEBUG
            // While the debug mode is on, the step is timed with os.clock for its statistics.
            var timed = DebugSession.IsOn;
            var started = timed ? DebugSession.Clock() : 0.0f;
#endif
            StepNow++;
            Running = true;
            var step = StepNow;
            try
            {
                var first = (step & (Slots - 1)) * Passes;
                var gizmos = Raw.At(ring, first);
                var checks = Raw.At(ring, first + 1);
                var interactions = Raw.At(ring, first + 2);
                var gizmoCount = gizmos.Count;
                var checkCount = checks.Count;
                var interactionCount = interactions.Count;
                RunPass(gizmos, gizmoCount, step);
                RunPass(checks, checkCount, step);
                RunPass(interactions, interactionCount, step);
                Pack(gizmos);
                Pack(checks);
                Pack(interactions);
            }
            finally
            {
                Running = false;
            }
#if DEBUG

            InteractionsView.StepRan(timed ? DebugSession.Clock() - started : 0.0f);
#endif
        }

        /// <summary>Runs, of the first count dues of a pass's list, those due at the step, in list order, each taken off before it runs; one with nothing more to run is taken off, and one due at a later turn of the ring stays.</summary>
        private void RunPass(List<IDue> list, int count, int step)
        {
            for (var i = 0; i < count; i++)
            {
                var due = Raw.At(list, i);
                if (due == taken)
                {
                    continue;
                }

                var at = due.DueStep;
                if (at < 0)
                {
                    Raw.Put(list, i, taken);
                }
                else if (at <= step)
                {
                    Raw.Put(list, i, taken);
                    due.RunDue(step);
                }
            }
        }

        /// <summary>Takes the markers out of a list, keeping the order of the rest; what was scheduled onto the list while it ran is at its end and stays.</summary>
        private void Pack(List<IDue> list)
        {
            var kept = 0;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var due = Raw.At(list, i);
                if (due != taken)
                {
                    Raw.Put(list, kept, due);
                    kept++;
                }
            }

            Raw.Truncate(list, kept);
        }

        /// <summary>The marker of a due taken off a list: no pass, never due, nothing to run.</summary>
        private sealed class Taken : IDue
        {
            /// <summary>No pass.</summary>
            public int Pass
            {
                get { return -1; }
            }

            /// <summary>Never due.</summary>
            public int DueStep
            {
                get { return -1; }
            }

            /// <summary>Runs nothing.</summary>
            public void RunDue(int step)
            {
            }
        }
    }
}
