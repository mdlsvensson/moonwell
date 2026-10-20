using Moonwell;

namespace OutOfRangeById
{
    public sealed class OutOfRangeByIdMap : Map
    {
    }

    public class Giant : Moonwell.Objects.Human.Units.Footman
    {
        public Giant()
        {
            Set("uhpm", 600000);
        }
    }
}
