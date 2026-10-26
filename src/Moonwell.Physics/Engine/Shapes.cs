namespace Moonwell.Engine
{
    /// <summary>The shapes the package sees things as. A unit is an upright cylinder: its collision size around, and its Body's Height tall (4 times its collision size unless set), from the ground plus its fly height.</summary>
    internal static class Shapes
    {
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
    }
}
