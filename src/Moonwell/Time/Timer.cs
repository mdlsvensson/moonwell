using System;

namespace Moonwell
{
    /// <summary>An action that runs once after a delay, or again at every interval, until it ends.</summary>
    public sealed class Timer : Scoped
    {
        /// <summary>The action each run calls.</summary>
        internal readonly Action action;

        /// <summary>The game time the timer was started at.</summary>
        internal readonly float start = 0.0f;

        /// <summary>The game time the timer is next due.</summary>
        internal float due = 0.0f;

        /// <summary>Whether the scheduler has called the action at least once.</summary>
        internal bool ran;

        /// <summary>
        /// How many intervals after its start a repeating timer is next due; 1 until it first runs. Its due time is worked out
        /// from the start and this count, not added up run by run, so the rounding of a float late in a game does not add up.
        /// </summary>
        internal int intervals = 1;

        /// <summary>Makes a timer due the seconds after its start, again at every interval of them when it repeats; it is not yet adopted or scheduled.</summary>
        internal Timer(float start, float seconds, bool periodic, Action action)
        {
            this.start = start;
            due = start + seconds;
            Interval = periodic ? seconds : 0.0f;
            IsPeriodic = periodic;
            this.action = action;
        }

        /// <summary>The seconds between runs of a repeating timer; 0 for a one-shot timer.</summary>
        [LocalSafe]
        public float Interval { get; } = 0.0f;

        /// <summary>Whether the timer runs again at every interval.</summary>
        [LocalSafe]
        public bool IsPeriodic { get; }

        /// <summary>The seconds until the timer is next due, inside a repeating timer's own run the time to its next run; 0 once it has ended.</summary>
        [LocalSafe]
        public float Remaining
        {
            get
            {
                return HasEnded ? 0.0f : due - Runtime.World.Now();
            }
        }

        /// <summary>
        /// Reports seconds of 0 or less given to a scope's Every, under the label, and gives back a timer that has ended without
        /// being adopted or scheduled. The error is thrown and caught here so that in Lua it records where it was thrown, which
        /// names the C# line that called Every.
        /// </summary>
        internal static Timer Refuse(string label, float seconds, Action action)
        {
            try
            {
                throw new MoonwellException("seconds must be more than 0, not " + seconds);
            }
            catch (MoonwellException error)
            {
                Errors.Report(label, error);
            }

            var timer = new Timer(Runtime.World.Now(), seconds, true, action);
            timer.End();
            return timer;
        }

#if DEBUG
        /// <summary>
        /// Reports, in a development build, a timer its scope ended in the instant it was made, before it ran: the line that
        /// made it named a scope already on its way out, so it does nothing, and nothing else says so. A timer ended by its own
        /// End is not reported.
        /// </summary>
        internal override void Finish()
        {
            if (!ran && Parent != null && Parent.ending && start == Runtime.World.Now())
            {
                Errors.Report(Names.NameOf(Parent) + ".Timer", new MoonwellException("ended with its scope in the instant it was made, before it ran; name a scope that lasts"));
            }
        }
#endif

        /// <summary>Adopts a new timer into its scope, and schedules it unless the scope has already ended.</summary>
        internal static Timer Start(Timer timer, Scope scope)
        {
            scope.Adopt(timer);
            if (!timer.HasEnded)
            {
                Runtime.Scheduler.Add(timer);
            }

            return timer;
        }
    }
}
