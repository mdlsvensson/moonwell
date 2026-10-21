using Moonwell;

namespace OutOfRange
{
    public sealed class OutOfRangeMap : Map
    {
    }

    public class Giant : Moonwell.Objects.Human.Units.Footman
    {
        public Giant()
        {
            MaxLife = 600000;
        }
    }
}
