using System;

namespace Moonwell
{
    /// <summary>An action that runs once after a delay, or again at every interval, until it ends.</summary>
    public sealed class Timer : Owned
    {
        /// <summary>The action each run calls.</summary>
        internal readonly Action action;

        /// <summary>The game time the timer is next due.</summary>
        internal float due;

        /// <summary>Makes a timer; it is not yet adopted or scheduled.</summary>
        private Timer(float due, float interval, bool periodic, Action action)
        {
            this.due = due;
            Interval = interval;
            IsPeriodic = periodic;
            this.action = action;
        }

        /// <summary>The seconds between runs of a repeating timer; 0 for a one-shot timer.</summary>
        [LocalSafe]
        public float Interval { get; }

        /// <summary>Whether the timer runs again at every interval.</summary>
        [LocalSafe]
        public bool IsPeriodic { get; }

        /// <summary>The seconds until the timer is next due; 0 once it has ended.</summary>
        [LocalSafe]
        public float Remaining
        {
            get
            {
                return HasEnded ? 0f : due - Runtime.World.Now();
            }
        }

        /// <summary>Runs an action once after the seconds, for the owner or else the current owner.</summary>
        public static Timer After(float seconds, Action action, Owner? owner = null)
        {
            Dev.Check(seconds >= 0f, "Timer.After: seconds must be 0 or more");
            return Start(new Timer(Runtime.World.Now() + seconds, 0f, false, action), owner);
        }

        /// <summary>Runs an action at every interval of the seconds, for the owner or else the current owner.</summary>
        public static Timer Every(float seconds, Action action, Owner? owner = null)
        {
            Dev.Check(seconds > 0f, "Timer.Every: seconds must be more than 0");
            return Start(new Timer(Runtime.World.Now() + seconds, seconds, true, action), owner);
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
