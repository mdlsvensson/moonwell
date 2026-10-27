namespace Moonwell.Testing
{
    /// <summary>
    /// A point a case moves a step at a time against shapes, as a map's bounce loop does: when the step's raycast hits, the
    /// point goes to the hit's point and its velocity bounces (or slides) off the hit's normal; else it goes to the step's end.
    /// </summary>
    public sealed class Bouncer
    {
        /// <summary>Where the point is.</summary>
        public Vector2 Position;

        /// <summary>How far it goes in a step.</summary>
        public Vector2 Velocity;

        /// <summary>Whether a hit slides it along the surface instead of bouncing it.</summary>
        public bool Slides;

        /// <summary>How many steps hit something.</summary>
        public int Hits;

        /// <summary>How many steps did not move it at all.</summary>
        public int Stuck;

        /// <summary>How far it has gone in all.</summary>
        public float Travelled;

        /// <summary>Makes a point at a place with a velocity.</summary>
        public Bouncer(Vector2 position, Vector2 velocity)
        {
            Position = position;
            Velocity = velocity;
        }

        /// <summary>Moves it one step: to the hit's point with the velocity bounced or slid there when it hit, else to the step's end.</summary>
        public void Step(bool hitSomething, RayHit hit)
        {
            var before = Position;
            if (hitSomething)
            {
                Hits++;
                Position = hit.Point;
                Velocity = Slides ? Velocity.Slide(hit.Normal) : Velocity.Bounce(hit.Normal);
            }
            else
            {
                Position = Position + Velocity;
            }

            var moved = Position.DistanceTo(before);
            if (moved == 0f)
            {
                Stuck++;
            }

            Travelled += moved;
        }
    }
}
