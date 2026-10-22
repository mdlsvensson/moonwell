using Moonwell;
using Moonwell.Objects;

namespace BeyondLimits
{
    public sealed class BeyondLimitsMap : Map
    {
    }

    [BeyondLimits(nameof(MaxLife))]
    public class Giant : Moonwell.Objects.Human.Units.Footman
    {
        public Giant()
        {
            MaxLife = 600000;
        }
    }
}
