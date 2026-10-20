using Moonwell;
using Moonwell.Objects;

namespace BeyondLimitsNegative
{
    public sealed class BeyondLimitsNegativeMap : Map
    {
    }

    [BeyondLimits(nameof(HitPoints))]
    public class Hollow : Moonwell.Objects.Human.Units.Footman
    {
        public Hollow()
        {
            HitPoints = -5;
        }
    }
}
