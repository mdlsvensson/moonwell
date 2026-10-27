using System;

namespace Moonwell
{
    /// <summary>An action that runs once after a delay, or again at every interval, until it ends.</summary>
    public sealed class Timer : Owned
    {
        /// <summary>The action each run calls.</summary>
        internal readonly Action action;

        /// <summary>The game time the timer was started at.</summary>
        internal readonly float start = 0.0f;

        /// <summary>The game time the timer is next due.</summary>
        internal float due = 0.0f;

        /// <summary>
        /// How many intervals after its start a repeating timer is next due; 1 until it first runs. Its due time is worked out
        /// from the start and this count, not added up run by run, so the rounding of a float late in a game does not add up.
        /// </summary>
        internal int intervals = 1;

        /// <summary>Makes a timer due the seconds after its start, again at every interval of them when it repeats; it is not yet adopted or scheduled.</summary>
        private Timer(float start, float seconds, bool periodic, Action action)
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

        /// <summary>The seconds until the timer is next due; 0 once it has ended.</summary>
        [LocalSafe]
        public float Remaining
        {
            get
            {
                return HasEnded ? 0.0f : due - Runtime.World.Now();
            }
        }

        /// <summary>Runs an action once after the seconds, for the owner or else the current owner.</summary>
        public static Timer After(float seconds, Action action, Owner? owner = null)
        {
            Dev.Check(seconds >= 0.0f, "Timer.After: seconds must be 0 or more");
            return Start(new Timer(Runtime.World.Now(), seconds, false, action), owner);
        }

        /// <summary>Runs an action at every interval of the seconds, for the owner or else the current owner.</summary>
        public static Timer Every(float seconds, Action action, Owner? owner = null)
        {
            Dev.Check(seconds > 0.0f, "Timer.Every: seconds must be more than 0");
            return Start(new Timer(Runtime.World.Now(), seconds, true, action), owner);
        }

        /// <summary>Adopts a new timer under its owner, and schedules it unless its owner has already ended.</summary>
        private static Timer Start(Timer timer, Owner? owner)
        {
            (owner ?? Owner.Current).Adopt(timer);
            if (!timer.HasEnded)
            {
                Runtime.Scheduler.Add(timer);
            }

            return timer;
        }
    }
}
