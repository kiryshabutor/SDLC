// Библиотека элементов интерфейса для макетов экранов (Приложение А, рисунки А.3–А.8).
// Имя части макета — видимый текст элемента; тип — вид элемента.
package UiKit {
    abstract part def Widget;

    // Контейнеры
    part def Screen :> Widget;   // экран целиком; дочерние элементы сверху вниз
    part def Column :> Widget;   // дочерние элементы сверху вниз
    part def Row :> Widget;      // дочерние элементы слева направо
    part def Form :> Column;     // форма ввода
    part def ListView :> Column; // список однотипных записей
    part def ListItem :> Row;    // запись списка

    // Элементы
    part def Label :> Widget;    // текст
    part def Input :> Widget;    // поле ввода; в скобках — пример значения
    part def Select :> Widget;   // выпадающий список
    part def Checkbox :> Widget; // флажок
    part def Button :> Widget;   // кнопка
    part def Link :> Widget;     // ссылка
    part def Image :> Widget;    // изображение
    part def Chart :> Widget;    // график
    part def Banner :> Widget;   // информационная плашка

    // Общие части экранов авторизованного пользователя (REQ-INT-002)
    part def NavBar :> Row;
    part def Footer :> Row;
}
