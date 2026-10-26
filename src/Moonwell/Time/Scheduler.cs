using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>One timer's place in the scheduler's heap: when it is due, its creation sequence, and the timer.</summary>
    internal sealed class TimerEntry
    {
        /// <summary>The game time the timer is due.</summary>
        internal readonly float due = 0.0f;

        /// <summary>The order the timer was made in; a repeating timer keeps it across repeats.</summary>
        internal readonly int sequence;

        /// <summary>The timer this entry is for.</summary>
        internal readonly Timer timer;

        /// <summary>Makes an entry for a timer.</summary>
        internal TimerEntry(float due, int sequence, Timer timer)
        {
            this.due = due;
            this.sequence = sequence;
            this.timer = timer;
        }
    }

    /// <summary>Runs the timers in due order, on one wake request to the world.</summary>
    internal sealed class Scheduler
    {
        /// <summary>The binary min-heap of timers, ordered by due time and then by creation.</summary>
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

        /// <summary>Runs every timer that is due, then asks the world to wake for the next one; in a development build, a timer that halts Moonwell's clock ends the run, and the timers due after it wait for the next step.</summary>
        internal void Wake()
        {
            var now = Runtime.World.Now();
#if DEBUG
            var wasHalted = Runtime.Clock.IsHalted;
#endif
            while (heap.Count > 0 && heap[0].due <= now + 0.001f)
            {
#if DEBUG
                if (!wasHalted && Runtime.Clock.IsHalted)
                {
                    break;
                }
#endif
                var entry = Pop();
                var timer = entry.timer;
                if (timer.HasEnded)
                {
                    continue;
                }

                Run(timer);
                if (!timer.IsPeriodic)
                {
                    timer.End();
                }
                else if (!timer.HasEnded)
                {
                    timer.due += timer.Interval;
                    Push(new TimerEntry(timer.due, entry.sequence, timer));
                }
            }

            Rearm();
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

        /// <summary>Calls a timer's action with its owner as the current owner, and reports an error it throws.</summary>
        private static void Run(Timer timer)
        {
            var reactor = timer.Owner!;
            var previous = Owner.current;
            Owner.current = reactor;
            try
            {
                timer.action();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(reactor) + ".Timer", error);
            }

            Owner.current = previous;
        }

        /// <summary>Whether the first entry is due before the second: by due time, then by creation.</summary>
        private static bool Before(TimerEntry first, TimerEntry second)
        {
            if (first.due < second.due - 0.001f)
            {
                return true;
            }

            if (second.due < first.due - 0.001f)
            {
                return false;
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
