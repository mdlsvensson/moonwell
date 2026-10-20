using Moonwell;
using Moonwell.Objects;

namespace BeyondLimits
{
    public sealed class BeyondLimitsMap : Map
    {
    }

    [BeyondLimits(nameof(HitPoints))]
    public class Giant : Moonwell.Objects.Human.Units.Footman
    {
        public Giant()
        {
            HitPoints = 600000;
        }
    }
}
