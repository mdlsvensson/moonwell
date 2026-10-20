using Moonwell;
using Moonwell.Objects;

namespace BeyondLimitsNegative
{
    public sealed class BeyondLimitsNegativeMap : Map
    {
    }

    [BeyondLimits(nameof(CollisionSize))]
    public class Hollow : Moonwell.Objects.Human.Units.Footman
    {
        public Hollow()
        {
            CollisionSize = -5f;
        }
    }
}
