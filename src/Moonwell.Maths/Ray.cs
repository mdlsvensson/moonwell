using System;

namespace Moonwell
{
    /// <summary>A start point and a direction of length 1. A readonly struct, as Vector2.</summary>
    public readonly struct Ray : IEquatable<Ray>
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

        /// <summary>Whether the origins and directions are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(Ray a, Ray b)
        {
            return a.Origin == b.Origin && a.Direction == b.Direction;
        }

        /// <summary>Whether the origin or the direction differs.</summary>
        [LocalSafe]
        public static bool operator !=(Ray a, Ray b)
        {
            return a.Origin != b.Origin || a.Direction != b.Direction;
        }

        /// <summary>Whether the origins and directions are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(Ray other)
        {
            return Origin == other.Origin && Direction == other.Direction;
        }

        /// <summary>Whether the object is a Ray with the same origin and direction.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is Ray other && Equals(other);
        }

        /// <summary>A hash of the origin and direction.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return Origin.GetHashCode() ^ (Direction.GetHashCode() * 397);
        }

        /// <summary>The ray as text: ((0, 0), (0, 1)).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + Origin + ", " + Direction + ")";
        }
    }
}
