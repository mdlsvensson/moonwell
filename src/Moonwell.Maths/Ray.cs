using System;

namespace Moonwell
{
    /// <summary>A start point and a direction of length 1. A readonly struct, as Vector2.</summary>
    public readonly struct Ray
    {
        /// <summary>Where the ray starts.</summary>
        public readonly Vector2 Origin;

        /// <summary>The direction, of length 1.</summary>
        public readonly Vector2 Direction;

        /// <summary>Makes a ray; the direction is normalized, and a zero direction throws an ArgumentException.</summary>
        [LocalSafe]
        public Ray(Vector2 origin, Vector2 direction)
        {
            if (direction.X == 0f && direction.Y == 0f)
            {
                throw new ArgumentException("A ray needs a direction.");
            }

            Origin = origin;
            Direction = direction.Normalized();
        }

        /// <summary>The point a distance along the ray.</summary>
        [LocalSafe]
        public Vector2 At(float distance)
        {
            return new Vector2(Origin.X + Direction.X * distance, Origin.Y + Direction.Y * distance);
        }
    }
}
