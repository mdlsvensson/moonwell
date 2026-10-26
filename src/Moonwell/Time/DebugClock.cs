#if DEBUG
namespace Moonwell
{
    /// <summary>
    /// Moonwell's own clock in a development build: game time underneath, which it can halt, step and run faster or slower.
    /// Each change rebases it: Moonwell's time at that game instant is kept, so its time never jumps.
    /// </summary>
    internal sealed class DebugClock
    {
        /// <summary>The game time of the last change.</summary>
        private float baseGame = 0.0f;

        /// <summary>Moonwell's time at the last change.</summary>
        private float baseTime = 0.0f;

        /// <summary>Whether the clock is halted: Moonwell's time stays at its time of the halt until a step or a resume.</summary>
        internal bool IsHalted { get; private set; }

        /// <summary>How many seconds of Moonwell's time pass in one second of game time.</summary>
        internal float Speed { get; private set; } = 1.0f;

        /// <summary>Moonwell's time at a game time.</summary>
        internal float Now(float gameNow)
        {
            if (IsHalted)
            {
                return baseTime;
            }

            return baseTime + (gameNow - baseGame) * Speed;
        }

        /// <summary>
        /// The game wait for a wake the seconds of Moonwell's time from now. False while halted: no wake is armed, and the
        /// caller of Resume, SetSpeed or Advance re-arms the request (Scheduler.Rearm).
        /// </summary>
        internal bool Arm(float seconds, out float gameSeconds)
        {
            if (IsHalted)
            {
                gameSeconds = 0.0f;
                return false;
            }

            gameSeconds = seconds / Speed;
            return true;
        }

        /// <summary>Stops Moonwell's time at its time at the game time; nothing if it is already halted.</summary>
        internal void Halt(float gameNow)
        {
            if (IsHalted)
            {
                return;
            }

            Rebase(gameNow);
            IsHalted = true;
        }

        /// <summary>Lets Moonwell's time run on from where it stopped, from the game time; nothing if it is not halted.</summary>
        internal void Resume(float gameNow)
        {
            if (!IsHalted)
            {
                return;
            }

            baseGame = gameNow;
            IsHalted = false;
        }

        /// <summary>Makes Moonwell's time run the speed times game time from the game time on.</summary>
        internal void SetSpeed(float speed, float gameNow)
        {
            Dev.Check(speed > 0.0f, "DebugClock.SetSpeed: speed must be more than 0");
            Rebase(gameNow);
            Speed = speed;
        }

        /// <summary>Moves Moonwell's time on by the seconds while halted: one step. The caller then wakes the runtime (Runtime.Wake).</summary>
        internal void Advance(float seconds)
        {
            Dev.Check(IsHalted, "DebugClock.Advance: the clock must be halted");
            Dev.Check(seconds >= 0.0f, "DebugClock.Advance: seconds must be 0 or more");
            baseTime += seconds;
        }

        /// <summary>Keeps Moonwell's time at the game time as the new base, so a change after it starts from there.</summary>
        private void Rebase(float gameNow)
        {
            baseTime = Now(gameNow);
            baseGame = gameNow;
        }
    }
}
#endif
