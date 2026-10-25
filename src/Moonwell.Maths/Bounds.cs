using System;

namespace Moonwell
{
    /// <summary>An axis-aligned rectangle on the map, from Min (lower left) to Max (upper right). A readonly struct, as Vector2. Named apart from the game's rect handle.</summary>
    public readonly struct Bounds : IEquatable<Bounds>
    {
        /// <summary>The lower-left corner.</summary>
        public readonly Vector2 Min;

        /// <summary>The upper-right corner.</summary>
        public readonly Vector2 Max;

        /// <summary>Makes bounds from any two opposite corners.</summary>
        [LocalSafe]
        public Bounds(Vector2 a, Vector2 b)
        {
            Min = new Vector2(MathF.Min(a.X, b.X), MathF.Min(a.Y, b.Y));
            Max = new Vector2(MathF.Max(a.X, b.X), MathF.Max(a.Y, b.Y));
        }

        /// <summary>The bounds of a size around a center.</summary>
        [LocalSafe]
        public static Bounds FromCenter(Vector2 center, Vector2 size)
        {
            var half = size / 2f;
            return new Bounds(center - half, center + half);
        }

        /// <summary>The middle.</summary>
        [LocalSafe]
        public Vector2 Center => new Vector2((Min.X + Max.X) / 2f, (Min.Y + Max.Y) / 2f);

        /// <summary>The width and height.</summary>
        [LocalSafe]
        public Vector2 Size => new Vector2(Max.X - Min.X, Max.Y - Min.Y);

        /// <summary>Whether a point is inside; the edges count as inside.</summary>
        [LocalSafe]
        public bool Contains(Vector2 point)
        {
            return point.X >= Min.X && point.X <= Max.X && point.Y >= Min.Y && point.Y <= Max.Y;
        }

        /// <summary>Whether two bounds overlap; touching counts.</summary>
        [LocalSafe]
        public bool Intersects(Bounds other)
        {
            return Min.X <= other.Max.X && Max.X >= other.Min.X && Min.Y <= other.Max.Y && Max.Y >= other.Min.Y;
        }

        /// <summary>The nearest point inside: a point inside is itself.</summary>
        [LocalSafe]
        public Vector2 Clamp(Vector2 point)
        {
            return new Vector2(Maths.Clamp(point.X, Min.X, Max.X), Maths.Clamp(point.Y, Min.Y, Max.Y));
        }

        /// <summary>The bounds grown by a distance on every side; a negative one shrinks them, never past a zero size at the center.</summary>
        [LocalSafe]
        public Bounds Expanded(float by)
        {
            var center = Center;
            var halfWidth = MathF.Max(0f, (Max.X - Min.X) / 2f + by);
            var halfHeight = MathF.Max(0f, (Max.Y - Min.Y) / 2f + by);
            return new Bounds(
                new Vector2(center.X - halfWidth, center.Y - halfHeight),
                new Vector2(center.X + halfWidth, center.Y + halfHeight));
        }

        /// <summary>Whether both corners are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(Bounds a, Bounds b)
        {
            return a.Min == b.Min && a.Max == b.Max;
        }

        /// <summary>Whether a corner differs.</summary>
        [LocalSafe]
        public static bool operator !=(Bounds a, Bounds b)
        {
            return a.Min != b.Min || a.Max != b.Max;
        }

        /// <summary>Whether both corners are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(Bounds other)
        {
            return Min == other.Min && Max == other.Max;
        }

        /// <summary>Whether the object is Bounds with both corners exactly equal.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is Bounds other && Equals(other);
        }

        /// <summary>A hash of both corners.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return Min.GetHashCode() ^ (Max.GetHashCode() * 397);
        }

        /// <summary>The bounds as text: ((0, 0), (10, 10)).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + Min + ", " + Max + ")";
        }
    }
}
