using Moonwell;

namespace GameCall
{
    public sealed class GameCallMap : Map
    {
    }

    public class Lucky : Moonwell.Objects.Units.Human.Footman
    {
        public Lucky()
        {
            HitPoints = Natives.GetRandomInt(1, 2);
        }
    }
}
