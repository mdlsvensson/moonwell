namespace Moonwell
{
    /// <summary>
    /// Map-wide reactions to each player's mouse, on the game's components (every player's mouse) and a player's (its own
    /// mouse); write only the ones needed, as public methods. The events are synced: every machine gets them at the same
    /// game time, so the reactions may change the game. Online they arrive after the network delay. Over the interface
    /// (the command card, the minimap, the portrait) they give the ground under it, so a click on a command button fires
    /// OnMousePressed too. A move does not follow the camera: the world moves under a still cursor and no event comes.
    /// </summary>
    public interface IMouseWatch
    {
        /// <summary>
        /// At each move event, up to 190 times a second for a player. The mouse's position is already the event's, unless
        /// the event read (0, 0) or the world's top-right corner less 32, which are not a place and keep the last one.
        /// </summary>
        /// @CSharpLua.Ignore
        void OnMouseMoved(Mouse mouse)
        {
        }

        /// <summary>When a button goes down. Over the interface (command card, minimap, portrait) it fires with the ground under it.</summary>
        /// @CSharpLua.Ignore
        void OnMousePressed(Mouse mouse, MouseButton button)
        {
        }

        /// <summary>When a button comes up.</summary>
        /// @CSharpLua.Ignore
        void OnMouseReleased(Mouse mouse, MouseButton button)
        {
        }
    }
}
