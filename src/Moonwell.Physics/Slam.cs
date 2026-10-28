namespace Moonwell
{
    /// <summary>A pushed unit running into something it cannot pass: how fast, where, the direction out of what it hit, and who pushed it. The package makes one at each slam and gives it to OnSlammed.</summary>
    public sealed class Slam
    {
        /// <summary>The speed lost into what it hit.</summary>
        internal float speed;

        /// <summary>The unit's place when it hit: x.</summary>
        internal float x;

        /// <summary>The unit's place when it hit: y.</summary>
        internal float y;

        /// <summary>The direction out of what it hit: x.</summary>
        internal float nx;

        /// <summary>The direction out of what it hit: y.</summary>
        internal float ny;

        /// <summary>The unit behind the push; null for none.</summary>
        internal Unit? pusher;

        /// <summary>Made by the package at a slam only.</summary>
        internal Slam(float speed, float x, float y, float nx, float ny, Unit? pusher)
        {
            this.speed = speed;
            this.x = x;
            this.y = y;
            this.nx = nx;
            this.ny = ny;
            this.pusher = pusher;
        }

        /// <summary>The speed the unit lost into what it hit, in game units a second: all of it when it stopped, the part into the edge when it slid along.</summary>
        [LocalSafe]
        public float Speed
        {
            get { return speed; }
        }

        /// <summary>The unit's left-to-right place when it hit; it stays there, or slides on from there.</summary>
        [LocalSafe]
        public float X
        {
            get { return x; }
        }

        /// <summary>The unit's bottom-to-top place when it hit.</summary>
        [LocalSafe]
        public float Y
        {
            get { return y; }
        }

        /// <summary>The direction out of what it hit, level and of length 1: against the push when it stopped, across the edge when it slid along; a vector is a table, made when read.</summary>
        [LocalSafe]
        public Vector3 Normal
        {
            get { return new Vector3(nx, ny, 0.0f); }
        }

        /// <summary>The unit behind the push (a missile's source), if any; null when there was none or it has left the game since.</summary>
        [LocalSafe]
        public Unit? Pusher
        {
            get { return pusher != null && !pusher.HasEnded && !pusher.IsGone ? pusher : null; }
        }
    }
}
