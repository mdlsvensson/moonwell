namespace Moonwell.Engine
{
    /// <summary>The shapes the package sees things as. A unit is an upright cylinder: its collision size around, and 4 times that tall, from the ground plus its fly height.</summary>
    internal static class Shapes
    {
        /// <summary>How many times its collision size a unit is tall.</summary>
        private const float HeightPerSize = 4.0f;

        /// <summary>A unit's radius: its collision size.</summary>
        public static float UnitRadius(Unit unit)
        {
            return Physics.World.CollisionSize(unit.Handle);
        }

        /// <summary>A unit's height: 4 times its collision size.</summary>
        public static float UnitHeight(Unit unit)
        {
            return HeightPerSize * UnitRadius(unit);
        }
    }
}
