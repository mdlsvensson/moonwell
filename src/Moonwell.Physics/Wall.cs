using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// A straight upright wall of the map's own, from one end to the other, as tall as its Height above the ground (or
    /// endless). Missiles whose WallContact is not None meet it, and a pushed unit is stopped by it, or slides along it, as
    /// at a tree. It is invisible: the map shows it however it likes. The package never moves it; setting its X or Y moves
    /// it whole. It belongs to its maker (the current owner at Create) unless given an owner, and ends with it or by End.
    /// </summary>
    public sealed class Wall : Gizmo
    {
        // The fields are internal, not private, as in Gizmo: a map's code cannot derive from it, but the package's own
        // classes keep to the one rule.

        /// <summary>The first end, x, where it was made.</summary>
        internal float x1;

        /// <summary>The first end, y, where it was made.</summary>
        internal float y1;

        /// <summary>The second end, x, where it was made.</summary>
        internal float x2;

        /// <summary>The second end, y, where it was made.</summary>
        internal float y2;

        /// <summary>Its place when it was made, x: the ends move by how far its place has moved since.</summary>
        internal float madeX;

        /// <summary>Its place when it was made, y.</summary>
        internal float madeY;

        /// <summary>How tall it is above the ground; 0 for endless.</summary>
        internal float height;

        /// <summary>What a bounce off it keeps of the missile's own Bounciness.</summary>
        internal float bounciness = 1.0f;

        /// <summary>Only Create makes a wall.</summary>
        internal Wall()
        {
        }

        /// <summary>The first end's left-to-right place.</summary>
        [LocalSafe]
        public float X1
        {
            get { return x1 + Shift(true); }
        }

        /// <summary>The first end's bottom-to-top place.</summary>
        [LocalSafe]
        public float Y1
        {
            get { return y1 + Shift(false); }
        }

        /// <summary>The second end's left-to-right place.</summary>
        [LocalSafe]
        public float X2
        {
            get { return x2 + Shift(true); }
        }

        /// <summary>The second end's bottom-to-top place.</summary>
        [LocalSafe]
        public float Y2
        {
            get { return y2 + Shift(false); }
        }

        /// <summary>How tall it is above the ground under it; 0 for endless. A missile above it, or a pushed unit lifted above it, goes over.</summary>
        [LocalSafe]
        public float Height
        {
            get { return height; }
            set { height = value; }
        }

        /// <summary>The share of a missile's own Bounciness a bounce off it keeps: the two multiplied. 1, the default, leaves the missile's as it is.</summary>
        [LocalSafe]
        public float Bounciness
        {
            get { return bounciness; }
            set { bounciness = value; }
        }

        /// <summary>
        /// Makes a straight upright wall from (x1, y1) to (x2, y2), as tall as the height above the ground (0, the default,
        /// for endless), with the bounciness a bounce off it keeps of a missile's own (1 by default). It belongs to the owner
        /// given, else to the current owner, and ends with it. The ends must differ, and the height must be 0 or more.
        /// </summary>
        public static Wall Create(float x1, float y1, float x2, float y2, float height = 0f, float bounciness = 1f, Owner? owner = null)
        {
            Dev.Check(x1 != x2 || y1 != y2, "Wall.Create: the two ends must differ");
            Dev.Check(height >= 0.0f, "Wall.Create: the height must be 0 or more");
            Physics.EnsureInstalled();
            var wall = new Wall();
            wall.x1 = x1;
            wall.y1 = y1;
            wall.x2 = x2;
            wall.y2 = y2;
            wall.height = height;
            wall.bounciness = bounciness;
            Gizmo.Create(wall, (x1 + x2) * 0.5f, (y1 + y2) * 0.5f, null, owner);
            return wall;
        }

        /// <summary>How far its place has moved since it was made, along x or along y.</summary>
        private float Shift(bool alongX)
        {
            if (entry == null)
            {
                return 0.0f;
            }

            return alongX ? entry.X - madeX : entry.Y - madeY;
        }

        /// <summary>Keeps its place as made and joins the package's walls, in the order they were made.</summary>
        internal override void Placed()
        {
            madeX = entry!.X;
            madeY = entry.Y;
            Walls.Add(this);
        }

        /// <summary>Tells the package's walls it is gone.</summary>
        internal override void Finish()
        {
            base.Finish();
            Walls.Dropped();
        }
    }
}
