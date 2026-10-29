using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>One timer's place in the scheduler's heap: when it is due, its creation sequence, and the timer.</summary>
    internal sealed class TimerEntry
    {
        /// <summary>The game time the timer is due.</summary>
        internal readonly float due = 0.0f;

        /// <summary>The due time in whole milliseconds, rounded: the heap's order before the sequence.</summary>
        internal readonly float millisecond = 0.0f;

        /// <summary>The order the timer was made in; a repeating timer keeps it across repeats.</summary>
        internal readonly int sequence;

        /// <summary>The timer this entry is for.</summary>
        internal readonly Timer timer;

        /// <summary>Makes an entry for a timer.</summary>
        internal TimerEntry(float due, int sequence, Timer timer)
        {
            this.due = due;
            millisecond = MathF.Floor(due * 1000.0f + 0.5f);
            this.sequence = sequence;
            this.timer = timer;
        }
    }

    /// <summary>Runs the timers in due order, on one wake request to the world.</summary>
    internal sealed class Scheduler
    {
        /// <summary>How many timer runs one instant (one wake) may have; the next is a runaway, reported, and the rest wait (decision 76).</summary>
        private const int RunawayRuns = 10000;

        /// <summary>The seconds the timers still due after a runaway wait, so the game goes on: 1/32 s.</summary>
        private const float RunawayWait = 1.0f / 32.0f;

        /// <summary>The binary min-heap of timers, ordered by due time in whole milliseconds and then by creation.</summary>
        private readonly List<TimerEntry> heap = new List<TimerEntry>();

        /// <summary>The sequence the next timer added gets.</summary>
        private int nextSequence;

        /// <summary>Puts a timer in the heap, and asks the world to wake when it is the next due.</summary>
        internal void Add(Timer timer)
        {
            var entry = new TimerEntry(timer.due, nextSequence, timer);
            nextSequence++;
            Push(entry);
            if (heap[0] == entry)
            {
                var wait = entry.due - Runtime.World.Now();
                if (wait < 0.0f)
                {
                    wait = 0.0f;
                }

                Runtime.World.WakeIn(wait);
            }
        }

        /// <summary>
        /// Runs every timer that is due, those made by the runs included (After(0) inside a timer runs in the same instant, as the
        /// game's own 0 s timer does), then asks the world to wake for the next one. More than RunawayRuns runs in one wake is a
        /// runaway, a timer that keeps starting itself, say: it is reported, naming the owner of the timer that would have run
        /// next, and the timers still due wait RunawayWait, so the game never freezes. In a development build, a timer that halts
        /// Moonwell's clock ends the run, and the timers due after it wait for the next step, and while the debug mode is on the
        /// run is timed for its statistics.
        /// </summary>
        internal void Wake()
        {
            var now = Runtime.World.Now();
            var runs = 0;
            var runaway = false;
#if DEBUG
            var wasHalted = Runtime.Clock.IsHalted;
            // While the debug mode is on, the wake is timed with os.clock for its statistics.
            var timed = DebugSession.IsOn;
            var started = timed ? DebugSession.Clock() : 0.0f;
#endif
            while (heap.Count > 0 && heap[0].due <= now + 0.001f)
            {
#if DEBUG
                if (!wasHalted && Runtime.Clock.IsHalted)
                {
                    break;
                }
#endif
                var entry = heap[0];
                var timer = entry.timer;
                if (timer.HasEnded)
                {
                    Pop();
                    continue;
                }

                if (runs == RunawayRuns)
                {
                    runaway = true;
                    Errors.Report(Names.NameOf(timer.Parent!) + ".Timer", new MoonwellException("more than 10000 timer runs in one instant; the rest wait 1/32 s"));
                    break;
                }

                Pop();
                runs++;
                if (timer.IsPeriodic)
                {
                    // Before the run, so Remaining inside it is the time to the next run.
                    MoveOn(timer);
                }

                Run(timer);
                if (!timer.IsPeriodic)
                {
                    timer.End();
                }
                else if (!timer.HasEnded)
                {
                    Push(new TimerEntry(timer.due, entry.sequence, timer));
                }
            }

#if DEBUG
            if (timed)
            {
                DebugStats.TimersRan(DebugSession.Clock() - started);
            }
#endif
            if (runaway)
            {
                Runtime.World.WakeIn(RunawayWait);
            }
            else
            {
                Rearm();
            }
        }

        /// <summary>Drops the ended timers from the top, and asks the world to wake for the next one, if any is left.</summary>
        internal void Rearm()
        {
            while (heap.Count > 0 && heap[0].timer.HasEnded)
            {
                Pop();
            }

            if (heap.Count > 0)
            {
                Runtime.World.WakeIn(heap[0].due - Runtime.World.Now());
            }
        }

        /// <summary>
        /// Moves a repeating timer on to its next run: due a whole number of intervals after its start, never before the run it
        /// moves on from. Late in a game a float moves in steps (0.0005 s from about 68 minutes); worked out from the start, the
        /// rounding of each due time stays within one step, and an interval shorter than a step gives runs that share an instant.
        /// In the game's own Lua an addition drops what is below two steps (seen 10-27), so there a due time can be up to two
        /// steps early, and runs share an instant from an interval of two steps.
        /// </summary>
        private static void MoveOn(Timer timer)
        {
            timer.intervals++;
            var next = timer.start + timer.intervals * timer.Interval;
            if (next > timer.due)
            {
                timer.due = next;
            }
        }

        /// <summary>Calls a timer's action with its owner as the current owner, and reports an error it throws.</summary>
        private static void Run(Timer timer)
        {
            var reactor = timer.Parent!;
            var previous = Scope.current;
            Scope.current = reactor;
            try
            {
                timer.action();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(reactor) + ".Timer", error);
            }

            Scope.current = previous;
        }

        /// <summary>
        /// Whether the first entry comes before the second: by due time in whole milliseconds, then by creation, so timers due in
        /// the same millisecond run in the order they were made (a buff's tick before an expiry due with it). Milliseconds are a
        /// fixed grid, so the order is transitive, which a tolerance between two due times is not; the tolerance is only in what
        /// is due (Wake).
        /// </summary>
        private static bool Before(TimerEntry first, TimerEntry second)
        {
            if (first.millisecond != second.millisecond)
            {
                return first.millisecond < second.millisecond;
            }

            return first.sequence < second.sequence;
        }

        /// <summary>Adds an entry at the end of the heap and moves it up to its place.</summary>
        private void Push(TimerEntry entry)
        {
            heap.Add(entry);
            var index = heap.Count - 1;
            while (index > 0)
            {
                var parent = (index - 1) / 2;
                if (!Before(heap[index], heap[parent]))
                {
                    break;
                }

                Swap(index, parent);
                index = parent;
            }
        }

        /// <summary>Takes the first entry out of the heap and moves the last one up to its place.</summary>
        private TimerEntry Pop()
        {
            var top = heap[0];
            var last = heap.Count - 1;
            heap[0] = heap[last];
            heap.RemoveAt(last);
            var index = 0;
            var count = heap.Count;
            while (true)
            {
                var left = index * 2 + 1;
                if (left >= count)
                {
                    break;
                }

                var smallest = left;
                var right = left + 1;
                if (right < count && Before(heap[right], heap[left]))
                {
                    smallest = right;
                }

                if (!Before(heap[smallest], heap[index]))
                {
                    break;
                }

                Swap(index, smallest);
                index = smallest;
            }

            return top;
        }

        /// <summary>Exchanges two entries of the heap.</summary>
        private void Swap(int first, int second)
        {
            var held = heap[first];
            heap[first] = heap[second];
            heap[second] = held;
        }
    }
}
