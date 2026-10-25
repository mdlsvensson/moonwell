using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>The package's one cycle: a ring of lists, one for each of the next 256 steps, of what is due. The step makes no table: the lists are reused, a due is taken off its list by being set to null, and the nulls are packed away at the end of the step.</summary>
    internal sealed class Cycle
    {
        /// <summary>How many steps the ring holds.</summary>
        private const int Slots = 256;

        /// <summary>How many passes a step has: gizmos, cell checks, interactions.</summary>
        private const int Passes = 3;

        /// <summary>The ring, a list of what is due for each step modulo 256, in the order it was scheduled.</summary>
        private readonly List<List<IDue?>> ring = new List<List<IDue?>>();

        /// <summary>The step the cycle is at: the one running, or the last one that ran. It counts up from 0 over the game, and does not reset when the cycle's timer stops.</summary>
        public int StepNow;

        /// <summary>Whether a step is running now. Outside one, StepNow is the last step that ran, and a place read between two steps is read again by the next ask.</summary>
        public bool Running;

        /// <summary>Makes a cycle at step 0 with an empty list for every slot.</summary>
        public Cycle()
        {
            for (var i = 0; i < Slots; i++)
            {
                ring.Add(new List<IDue?>());
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

        /// <summary>Puts a due on the list of a step, which must be after the one running, so that what a step schedules never runs in that step. A due further than 255 steps ahead waits in its slot and is passed over until its step.</summary>
        public void Schedule(IDue due, int step)
        {
            Dev.Check(step > StepNow, "Cycle.Schedule: a due must be scheduled for a step after the one running");
            ring[step % Slots].Add(due);
        }

        /// <summary>Runs the next step: what is due at it, in three passes, each in the order it was scheduled. A due that is not due until a later turn of the ring is passed over, and one with nothing more to run is dropped.</summary>
        public void Run()
        {
            StepNow++;
            Running = true;
            var step = StepNow;
            try
            {
                var list = ring[step % Slots];
                var count = list.Count;
                for (var pass = 0; pass < Passes; pass++)
                {
                    for (var i = 0; i < count; i++)
                    {
                        var due = list[i];
                        if (due == null || due.Pass != pass)
                        {
                            continue;
                        }

                        var at = due.DueStep;
                        if (at < 0)
                        {
                            list[i] = null;
                        }
                        else if (at <= step)
                        {
                            list[i] = null;
                            due.RunDue(step);
                        }
                    }
                }

                Pack(list);
            }
            finally
            {
                Running = false;
            }
        }

        /// <summary>Takes the nulls out of a list, keeping the order of the rest; what was scheduled onto the list while it ran is at its end and stays.</summary>
        private static void Pack(List<IDue?> list)
        {
            var kept = 0;
            for (var i = 0; i < list.Count; i++)
            {
                var due = list[i];
                if (due != null)
                {
                    list[kept] = due;
                    kept++;
                }
            }

            while (list.Count > kept)
            {
                list.RemoveAt(list.Count - 1);
            }
        }
    }
}
