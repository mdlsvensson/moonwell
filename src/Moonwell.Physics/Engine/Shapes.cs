namespace Moonwell.Engine
{
    /// <summary>
    /// The shapes the package sees things as, each an upright cylinder from the ground: a unit is its collision size around
    /// and its Body's Height tall (4 times its collision size unless set), from the ground plus its fly height; a
    /// destructable is 64 around and 250 tall unless Physics.DestructableSize sized its type; an item is 40 around and 40
    /// tall. A missile is a ball of its Radius.
    /// </summary>
    internal static class Shapes
    {
        /// <summary>A destructable's radius unless its type was sized.</summary>
        public const float DestructableRadius = 64.0f;

        /// <summary>A destructable's height unless its type was sized.</summary>
        public const float DestructableHeight = 250.0f;

        /// <summary>An item's radius.</summary>
        public const float ItemRadius = 40.0f;

        /// <summary>An item's height.</summary>
        public const float ItemHeight = 40.0f;

        /// <summary>A unit's radius: its collision size.</summary>
        public static float UnitRadius(Unit unit)
        {
            return Physics.World.CollisionSize(unit.Handle);
        }

        /// <summary>A unit's height: its Body's Height, else 4 times its collision size.</summary>
        public static float UnitHeight(Unit unit)
        {
            var body = unit.Get<Body>();
            if (body != null && !body.HasEnded)
            {
                return body.Height;
            }

            return Body.HeightPerSize * UnitRadius(unit);
        }

        /// <summary>A destructable's radius and height: those Physics.DestructableSize gave its type, else 64 and 250.</summary>
        public static void DestructableSize(Destructable destructable, out float radius, out float height)
        {
            var settings = Physics.Current();
            var radii = settings.DestructableRadii;
            var heights = settings.DestructableHeights;
            var type = destructable.TypeId;
            if (radii != null && heights != null && radii.TryGetValue(type, out radius))
            {
                height = heights[type];
                return;
            }

            radius = DestructableRadius;
            height = DestructableHeight;
        }
    }
}
